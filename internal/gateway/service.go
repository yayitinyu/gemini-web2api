package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/gemini"
	"github.com/yayitinyu/gemini-web2api/internal/store"
)

const maxAccountAttempts = 3

type Service struct {
	store  *store.Store
	client *gemini.Client
}

type GenerateInput struct {
	Model       string
	Prompt      string
	OnContent   func(string)
	OnReasoning func(string)
}

type Execution struct {
	RequestedModel string
	Account        *store.Account
	Proxy          *store.Proxy
	Result         gemini.Result
}

type ContextLengthError struct {
	Bytes int
	Limit int
}

func (e *ContextLengthError) Error() string {
	return fmt.Sprintf("prompt is %d UTF-8 bytes, over the configured %d-byte safety limit; shorten or summarize the conversation", e.Bytes, e.Limit)
}

type NoUsableAccountError struct {
	Attempts int
	Last     error
}

func (e *NoUsableAccountError) Error() string {
	if e.Last == nil {
		return "no enabled Google account is available"
	}
	return fmt.Sprintf("no usable Google account after %d attempt(s): %v", e.Attempts, e.Last)
}

type ProxyUnavailableError struct{}

func (*ProxyUnavailableError) Error() string {
	return "proxy pool is configured but no egress is currently available; reset a cooled-down proxy or enable direct fallback"
}

func New(st *store.Store, client *gemini.Client) *Service {
	return &Service{store: st, client: client}
}

func (s *Service) Models(ctx context.Context) ([]gemini.Model, error) {
	_, enabled, err := s.store.AccountCounts(ctx)
	if err != nil {
		return nil, err
	}
	return gemini.Models(enabled > 0), nil
}

func (s *Service) Generate(ctx context.Context, input GenerateInput) (Execution, error) {
	settings, err := s.store.RuntimeSettings(ctx)
	if err != nil {
		return Execution{}, err
	}
	if input.Model == "" {
		input.Model = settings.DefaultModel
	}
	if settings.MaxPromptBytes > 0 && len(input.Prompt) > settings.MaxPromptBytes {
		return Execution{RequestedModel: input.Model}, &ContextLengthError{Bytes: len(input.Prompt), Limit: settings.MaxPromptBytes}
	}
	_, enabledAccounts, err := s.store.AccountCounts(ctx)
	if err != nil {
		return Execution{}, err
	}
	model, err := gemini.ResolveModel(input.Model, enabledAccounts > 0)
	if err != nil {
		return Execution{RequestedModel: input.Model}, err
	}

	execution := Execution{RequestedModel: model.ID}
	excluded := make(map[int64]struct{})
	var lastAccountErr error
	for attempt := 0; attempt < maxAccountAttempts; attempt++ {
		account, hasAccount, err := s.store.PickAccountExcluding(ctx, excluded)
		if err != nil {
			return execution, err
		}
		if hasAccount {
			excluded[account.ID] = struct{}{}
			execution.Account = &account
		} else {
			execution.Account = nil
		}

		if model.RequiresAuth && !hasAccount {
			return execution, &NoUsableAccountError{Attempts: len(excluded), Last: lastAccountErr}
		}
		proxy, hasProxy, err := s.pickProxy(ctx, settings, execution.Account)
		if err != nil {
			return execution, err
		}
		if hasProxy {
			execution.Proxy = &proxy
		} else {
			execution.Proxy = nil
		}

		credential := gemini.Credential{}
		if execution.Account != nil {
			credential.Cookie = execution.Account.Cookie
			credential.SAPISID = gemini.ExtractSAPISID(execution.Account.Cookie)
		}
		proxyURL := ""
		if execution.Proxy != nil {
			proxyURL = execution.Proxy.URL
		}
		result, generateErr := s.client.Generate(ctx, gemini.GenerateRequest{
			Prompt: input.Prompt, Model: model, Credential: credential, ProxyURL: proxyURL,
			Options: gemini.Options{
				Timeout:       time.Duration(settings.RequestTimeoutSec) * time.Second,
				RetryAttempts: settings.RetryAttempts,
				RetryDelay:    time.Duration(settings.RetryDelayMS) * time.Millisecond,
				PinnedBL:      settings.GeminiBL,
				AutoRefreshBL: settings.GeminiBLAuto,
			},
			OnContent: input.OnContent, OnReasoning: input.OnReasoning,
		})
		execution.Result = result
		if execution.Proxy != nil && (generateErr == nil || !isCredentialError(generateErr)) {
			_ = s.store.ReportProxy(context.WithoutCancel(ctx), execution.Proxy.ID, generateErr == nil, errorString(generateErr))
		}
		if generateErr == nil {
			if execution.Account != nil {
				_ = s.store.ReportAccount(context.WithoutCancel(ctx), execution.Account.ID, true, "")
				if len(result.SetCookies) > 0 {
					merged := gemini.MergeSetCookies(execution.Account.Cookie, result.SetCookies)
					if merged != execution.Account.Cookie {
						_ = s.store.UpdateAccountCookie(context.WithoutCancel(ctx), execution.Account.ID, merged)
					}
				}
			}
			return execution, nil
		}

		if execution.Account == nil || !isCredentialError(generateErr) || result.EmittedText != "" || result.EmittedReasoning != "" {
			return execution, generateErr
		}
		_ = s.store.ReportAccount(context.WithoutCancel(ctx), execution.Account.ID, false, generateErr.Error())
		lastAccountErr = generateErr
		if len(excluded) >= int(enabledAccounts) {
			break
		}
	}

	if settings.FallbackAnonymous && !model.RequiresAuth {
		execution.Account = nil
		proxyURL := ""
		if execution.Proxy != nil {
			proxyURL = execution.Proxy.URL
		}
		result, err := s.client.Generate(ctx, gemini.GenerateRequest{
			Prompt: input.Prompt, Model: model, ProxyURL: proxyURL,
			Options: gemini.Options{
				Timeout: time.Duration(settings.RequestTimeoutSec) * time.Second, RetryAttempts: settings.RetryAttempts,
				RetryDelay: time.Duration(settings.RetryDelayMS) * time.Millisecond, PinnedBL: settings.GeminiBL,
				AutoRefreshBL: settings.GeminiBLAuto,
			},
			OnContent: input.OnContent, OnReasoning: input.OnReasoning,
		})
		execution.Result = result
		if execution.Proxy != nil {
			_ = s.store.ReportProxy(context.WithoutCancel(ctx), execution.Proxy.ID, err == nil, errorString(err))
		}
		return execution, err
	}
	return execution, &NoUsableAccountError{Attempts: len(excluded), Last: lastAccountErr}
}

type ProbeInput struct {
	Model     string
	Prompt    string
	AccountID int64
	ProxyID   int64
}

func (s *Service) Probe(ctx context.Context, input ProbeInput) (Execution, error) {
	settings, err := s.store.RuntimeSettings(ctx)
	if err != nil {
		return Execution{}, err
	}
	if input.Model == "" {
		input.Model = settings.DefaultModel
	}
	if input.Prompt == "" {
		input.Prompt = "Reply with exactly: OK"
	}
	execution := Execution{RequestedModel: input.Model}
	credential := gemini.Credential{}
	if input.AccountID > 0 {
		account, err := s.store.Account(ctx, input.AccountID, true)
		if err != nil {
			return execution, err
		}
		execution.Account = &account
		credential = gemini.Credential{Cookie: account.Cookie, SAPISID: gemini.ExtractSAPISID(account.Cookie)}
		if input.ProxyID == 0 {
			input.ProxyID = account.ProxyID
		}
	}
	model, err := gemini.ResolveModel(input.Model, execution.Account != nil)
	if err != nil {
		return execution, err
	}
	proxyURL := ""
	if input.ProxyID > 0 {
		proxy, err := s.store.Proxy(ctx, input.ProxyID, true)
		if err != nil {
			return execution, err
		}
		execution.Proxy = &proxy
		proxyURL = proxy.URL
	}
	result, generateErr := s.client.Generate(ctx, gemini.GenerateRequest{
		Prompt: input.Prompt, Model: model, Credential: credential, ProxyURL: proxyURL,
		Options: gemini.Options{
			Timeout: time.Duration(settings.RequestTimeoutSec) * time.Second, RetryAttempts: 1,
			PinnedBL: settings.GeminiBL, AutoRefreshBL: settings.GeminiBLAuto,
		},
	})
	execution.Result = result
	if execution.Account != nil {
		_ = s.store.ReportAccount(context.WithoutCancel(ctx), execution.Account.ID, generateErr == nil, errorString(generateErr))
	}
	if execution.Proxy != nil && (generateErr == nil || !isCredentialError(generateErr)) {
		_ = s.store.ReportProxy(context.WithoutCancel(ctx), execution.Proxy.ID, generateErr == nil, errorString(generateErr))
	}
	return execution, generateErr
}

func (s *Service) pickProxy(ctx context.Context, settings store.RuntimeSettings, account *store.Account) (store.Proxy, bool, error) {
	_, enabled, err := s.store.ProxyCounts(ctx)
	if err != nil || enabled == 0 {
		return store.Proxy{}, false, err
	}
	preferred := int64(0)
	if account != nil {
		preferred = account.ProxyID
	}
	proxy, ok, err := s.store.PickProxy(ctx, preferred)
	if err != nil {
		return store.Proxy{}, false, err
	}
	if !ok {
		if settings.FallbackDirect {
			return store.Proxy{}, false, nil
		}
		return store.Proxy{}, false, &ProxyUnavailableError{}
	}
	if account != nil && account.ProxyID == 0 {
		_ = s.store.BindAccountProxy(context.WithoutCancel(ctx), account.ID, proxy.ID)
	}
	return proxy, true, nil
}

func isCredentialError(err error) bool {
	if err == nil {
		return false
	}
	var upstream *gemini.UpstreamError
	if errors.As(err, &upstream) && (upstream.StatusCode == 401 || upstream.StatusCode == 403) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "signed-in cookie") || strings.Contains(message, "xsrf token")
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
