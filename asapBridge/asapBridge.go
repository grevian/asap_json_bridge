package asapBridge

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var defaultRefreshPeriod = time.Minute * 10

var errCertRefresh = errors.New(`failed to refresh certificates`)

type asapBridge struct {
	jsonCertPath     string
	refreshPeriod    time.Duration
	cachedCertMap    map[string]string
	cacheMutex       sync.RWMutex
	lifecycleContext context.Context
}

func NewAsapBridge(ctx context.Context, jsonCertPath string) (*asapBridge, error) {
	ab := &asapBridge{
		jsonCertPath:     jsonCertPath,
		refreshPeriod:    defaultRefreshPeriod,
		lifecycleContext: ctx,
	}

	certMap, err := ab.getCerts()
	if err != nil {
		return nil, err
	}
	ab.cachedCertMap = certMap
	go ab.refreshCerts()

	return ab, nil
}

func (a *asapBridge) getCerts() (map[string]string, error) {
	requestContext, _ := context.WithTimeout(a.lifecycleContext, time.Second*10)
	refreshRequest, err := http.NewRequestWithContext(requestContext, http.MethodGet, a.jsonCertPath, nil)
	if err != nil {
		return nil, err
	}
	certResponse, err := http.DefaultClient.Do(refreshRequest)
	if err != nil {
		return nil, err
	}

	if certResponse.StatusCode != http.StatusOK {
		return nil, errCertRefresh
	}

	bodyBytes, err := ioutil.ReadAll(certResponse.Body)
	if err != nil {
		return nil, err
	}

	certMap := make(map[string]string)
	err = json.Unmarshal(bodyBytes, &certMap)
	if err != nil {
		return nil, err
	}

	return certMap, nil
}

func (a *asapBridge) refreshCerts() {
	entry := logrus.WithFields(logrus.Fields{
		`certPath`: a.jsonCertPath,
		`refreshPeriod`: a.refreshPeriod,
	})

	refreshTicker := time.NewTicker(a.refreshPeriod)
	for {
		select {
		case <-refreshTicker.C:
			certMap, err := a.getCerts()
			if err != nil {
				entry.WithError(err).Warn(`failed to refresh certificates`)
			}
			a.cacheMutex.Lock()
			a.cachedCertMap = certMap
			a.cacheMutex.Unlock()
			entry.Info(`refreshed certificates`)
		case <-a.lifecycleContext.Done():
			return
		}
	}
}

func (a *asapBridge) ServeCert(w http.ResponseWriter, r *http.Request) {
	entry := logrus.WithFields(logrus.Fields{
		`certPath`: a.jsonCertPath,
		`refreshPeriod`: a.refreshPeriod,
	})

	vars := mux.Vars(r)
	certId := vars["certId"]
	a.cacheMutex.RLock()
	defer a.cacheMutex.RUnlock()
	if cert, exists := a.cachedCertMap[certId]; !exists {
		entry.WithField(`certId`, certId).Warn(`requested certId not found`)
		w.WriteHeader(http.StatusNotFound)
	} else {
		entry.WithField(`certId`, certId).Debug(`requested certId found`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(cert))
	}
}
