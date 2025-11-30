package hamnut

import (
	"fmt"
	"github.com/Station-Manager/config"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/types"
	"github.com/Station-Manager/utils"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ServiceName = types.HamNutLookupServiceName
)

type Service struct {
	LoggerService *logging.Service `di.inject:"logger"`
	ConfigService *config.Service  `di.inject:"config"`
	Config        *types.LookupConfig
	client        *http.Client

	isInitialized atomic.Bool
	initOnce      sync.Once
}

func (s *Service) Initialize() error {
	const op errors.Op = "hamnut.Service.Initialize"
	if s.isInitialized.Load() {
		return nil
	}

	var initErr error
	s.initOnce.Do(func() {
		if s.LoggerService == nil {
			initErr = errors.New(op).Msg("logger service has not been set/injected")
			return
		}
		if s.ConfigService == nil {
			initErr = errors.New(op).Msg("application config has not been set/injected")
			return
		}

		cfg, err := s.ConfigService.LookupServiceConfig(ServiceName)
		if err != nil {
			initErr = errors.New(op).Err(err).Msg("getting lookup service config")
			return
		}
		s.Config = &cfg
		s.client = utils.NewHTTPClient(s.Config.HttpTimeout * time.Second)

		s.isInitialized.Store(true)
	})

	return initErr
}

func (s *Service) Lookup(callsign string) (types.Country, error) {
	const op errors.Op = "hamnut.Service.Lookup"
	emptyRetVal := types.Country{}
	if !s.isInitialized.Load() {
		return emptyRetVal, errors.New(op).Msg("service is not initialized")
	}
	if s.Config != nil && !s.Config.Enabled {
		s.LoggerService.InfoWith().Msg("Hamnut callsign/prefix lookup is disabled in the config")
		return types.Country{Name: callsign}, nil
	}

	callsign = strings.TrimSpace(callsign)
	if callsign == "" {
		return types.Country{}, errors.New(op).Msg("callsign cannot be empty")
	}

	params := url.Values{
		"prefix": {callsign},
	}

	theUrl := fmt.Sprintf("%s?%s", s.Config.URL, params.Encode())
	req, err := http.NewRequest(http.MethodGet, theUrl, nil)
	if err != nil {
		err = errors.New(op).Err(err).Msg("Failed to create HTTP GET request")
		return types.Country{}, err
	}

	req.Header.Set("User-Agent", s.Config.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		err = errors.New(op).Err(err).Msg("Failed to perform HTTP GET request")
		return types.Country{}, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return types.Country{}, errors.New(op).Err(errors.ErrNotFound).Msg("Prefix not found by Hamnut")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		err = errors.New(op).Errorf("Service returned unexpected status %d: %s", resp.StatusCode, string(b))
		return types.Country{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = errors.New(op).Errorf("Failed to read response body: %w", err)
		return types.Country{}, err
	}

	country, err := s.unmarshalResponse(body)
	if err != nil {
		err = errors.New(op).Errorf("Failed to unmarshal JSON: %w", err)
		return types.Country{}, err
	}

	return country, nil
}
