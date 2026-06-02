package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

// webIdleTimeout is how long the server waits for the user to submit before giving up.
const webIdleTimeout = 5 * time.Minute

// webPostGrace is how long the server stays alive after a successful save so the
// browser tab can render the success page before the process exits.
const webPostGrace = 2 * time.Second

// formData drives the HTML template.
type formData struct {
	Token           string
	Error           string
	DefaultProvider string
	SameKey         bool
	OpenAI          providerFormFields
	Anthropic       providerFormFields
}

type providerFormFields struct {
	Enabled bool
	BaseURL string
	Model   string
	HasKey  bool
}

// setupWeb is the user-facing entry point: it picks a free port, generates a
// one-shot token, kills any prior worker, prints the URL, opens the user's
// browser, then spawns a detached worker child to actually serve the form.
// Returns sub-100ms — the worker holds the server until save or timeout.
func setupWeb() error {
	noOpen := false
	for _, a := range os.Args {
		if a == "--no-open" {
			noOpen = true
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return xerrors.Internal("listen", "cannot bind to 127.0.0.1", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		_ = ln.Close()
		return xerrors.Internal("rand", "cannot generate token", err)
	}
	token := hex.EncodeToString(tokenBytes)

	_ = killOldSetupWebWorker()

	url := fmt.Sprintf("http://127.0.0.1:%d/?t=%s", port, token)
	fmt.Println("Open this in your browser to finish setup:")
	fmt.Println(url)

	if !noOpen {
		openInBrowser(url)
	}

	pid, err := spawnSetupWebWorker(ln, token)
	// Parent never serves on the listener — close its own ref so only the worker holds it.
	_ = ln.Close()
	if err != nil {
		return err
	}

	if err := writeSetupWebPID(pid); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot write PID file: %v\n", err)
	}
	return nil
}

// serveSetupWeb runs the HTTP handlers against ln (created by the parent and
// inherited from fd 3 in the worker). Returns when a save completes, the
// idle timeout fires, or a signal arrives.
func serveSetupWeb(ln net.Listener, token string) error {
	formTmpl, err := template.New("form").Parse(setupFormHTML)
	if err != nil {
		return xerrors.Internal("parse_template", "cannot parse form template", err)
	}

	saved := make(chan struct{}, 1)
	var savedFlag atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") != token {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		renderForm(w, formTmpl, token, "", loadInitialFormData())
	})
	mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") != token {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if savedFlag.Load() {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(setupSuccessHTML))
			return
		}
		fd, errMsg := parseForm(r.PostForm)
		fd.Token = token
		if errMsg != "" {
			fd.Error = errMsg
			renderForm(w, formTmpl, token, errMsg, fd)
			return
		}
		if err := writeConfigFromForm(fd, r.PostForm); err != nil {
			fd.Error = err.Error()
			renderForm(w, formTmpl, token, err.Error(), fd)
			return
		}
		savedFlag.Store(true)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(setupSuccessHTML))
		select {
		case saved <- struct{}{}:
		default:
		}
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-saved:
		time.Sleep(webPostGrace)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case <-time.After(webIdleTimeout):
		_ = srv.Close()
		return xerrors.User("timeout", "setup timed out without a save", "")
	case <-sigCh:
		_ = srv.Close()
		return xerrors.User("cancelled", "setup cancelled by signal", "")
	}
}

// loadInitialFormData reads the current config (if any) and returns initial form values.
func loadInitialFormData() formData {
	fd := formData{
		DefaultProvider: "openai",
		OpenAI: providerFormFields{
			Enabled: true,
			BaseURL: config.DefaultOpenAIBaseURL,
			Model:   config.DefaultOpenAIModel,
		},
		Anthropic: providerFormFields{
			Enabled: false,
			BaseURL: config.DefaultAnthropicBaseURL,
			Model:   config.DefaultAnthropicModel,
		},
	}
	file, err := config.LoadFile()
	if err != nil {
		return fd
	}
	file = config.MigrateLegacy(file)
	if file.DefaultProvider != "" {
		fd.DefaultProvider = file.DefaultProvider
	}
	if file.OpenAI != nil {
		fd.OpenAI.Enabled = true
		if file.OpenAI.BaseURL != "" {
			fd.OpenAI.BaseURL = file.OpenAI.BaseURL
		}
		if file.OpenAI.Model != "" {
			fd.OpenAI.Model = file.OpenAI.Model
		}
		fd.OpenAI.HasKey = file.OpenAI.APIKey != ""
	}
	if file.Anthropic != nil {
		fd.Anthropic.Enabled = true
		if file.Anthropic.BaseURL != "" {
			fd.Anthropic.BaseURL = file.Anthropic.BaseURL
		}
		if file.Anthropic.Model != "" {
			fd.Anthropic.Model = file.Anthropic.Model
		}
		fd.Anthropic.HasKey = file.Anthropic.APIKey != ""
	}
	return fd
}

// renderForm renders the form template. errMsg goes into the banner (empty = no banner).
func renderForm(w http.ResponseWriter, tmpl *template.Template, token, errMsg string, fd formData) {
	fd.Token = token
	fd.Error = errMsg
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, fd)
}

// parseForm reads the form values into a formData (preserving them for re-render)
// and returns the first validation error (if any). It does NOT include API keys
// in the returned formData — keys are pulled separately during write.
func parseForm(values url.Values) (formData, string) {
	openaiEnabled := values.Get("openai_enabled") == "1"
	anthropicEnabled := values.Get("anthropic_enabled") == "1"

	fd := formData{
		DefaultProvider: values.Get("default_provider"),
		SameKey:         values.Get("same_key") == "1",
		OpenAI: providerFormFields{
			Enabled: openaiEnabled,
			BaseURL: strings.TrimSpace(values.Get("openai_base_url")),
			Model:   strings.TrimSpace(values.Get("openai_model")),
		},
		Anthropic: providerFormFields{
			Enabled: anthropicEnabled,
			BaseURL: strings.TrimSpace(values.Get("anthropic_base_url")),
			Model:   strings.TrimSpace(values.Get("anthropic_model")),
		},
	}

	if !openaiEnabled && !anthropicEnabled {
		return fd, "Pick at least one provider to configure."
	}
	if openaiEnabled {
		if err := validateProviderFields("OpenAI-compatible", fd.OpenAI.BaseURL, fd.OpenAI.Model); err != nil {
			return fd, err.Error()
		}
	}
	if anthropicEnabled {
		if err := validateProviderFields("Anthropic-compatible", fd.Anthropic.BaseURL, fd.Anthropic.Model); err != nil {
			return fd, err.Error()
		}
	}
	switch fd.DefaultProvider {
	case "openai":
		if !openaiEnabled {
			return fd, "Default provider is openai but OpenAI-compatible is not configured."
		}
	case "anthropic":
		if !anthropicEnabled {
			return fd, "Default provider is anthropic but Anthropic-compatible is not configured."
		}
	default:
		return fd, "Pick a default provider (openai or anthropic)."
	}
	return fd, ""
}

func validateProviderFields(label, baseURL, model string) error {
	if baseURL == "" {
		return fmt.Errorf("%s: base URL is required.", label)
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s: base URL is not a valid URL.", label)
	}
	if model == "" {
		return fmt.Errorf("%s: model is required.", label)
	}
	return nil
}

// writeConfigFromForm reads the (already-validated) form, merges with existing
// config (reusing keys where the form left the key field blank), and saves.
func writeConfigFromForm(fd formData, values url.Values) error {
	existing, _ := config.LoadFile()
	existing = config.MigrateLegacy(existing)

	out := config.File{
		DefaultProvider: fd.DefaultProvider,
	}

	if fd.OpenAI.Enabled {
		key := strings.TrimSpace(values.Get("openai_api_key"))
		if key == "" {
			if existing.OpenAI != nil {
				key = existing.OpenAI.APIKey
			}
		}
		if key == "" {
			return errors.New("OpenAI-compatible: API key is required (no existing key to reuse).")
		}
		out.OpenAI = &config.ProviderEntry{
			BaseURL: fd.OpenAI.BaseURL,
			Model:   fd.OpenAI.Model,
			APIKey:  key,
		}
	}

	if fd.Anthropic.Enabled {
		key := strings.TrimSpace(values.Get("anthropic_api_key"))
		if key == "" && fd.SameKey && fd.OpenAI.Enabled {
			// SameKey: take the openai key the form submitted (or the reused one).
			if out.OpenAI != nil {
				key = out.OpenAI.APIKey
			}
		}
		if key == "" {
			if existing.Anthropic != nil {
				key = existing.Anthropic.APIKey
			}
		}
		if key == "" {
			return errors.New("Anthropic-compatible: API key is required (no existing key to reuse).")
		}
		out.Anthropic = &config.ProviderEntry{
			BaseURL: fd.Anthropic.BaseURL,
			Model:   fd.Anthropic.Model,
			APIKey:  key,
		}
	}

	if err := config.Save(out); err != nil {
		return fmt.Errorf("cannot save config: %v", err)
	}
	return nil
}
