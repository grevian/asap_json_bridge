package main

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"asap_json_bridge/asapBridge"
)

func main() {
	ab, err := asapBridge.NewAsapBridge(context.Background(), `https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com`)
	if err != nil {
		logrus.WithError(err).Fatal(`could not construct asap bridge`)
	}

	r := mux.NewRouter()
	r.HandleFunc(`/keys/{certId}.pem`, ab.ServeCert)
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.WithFields(logrus.Fields{
			`request`: r,
			}).Error(`unhandled path requested`)
		w.WriteHeader(http.StatusNotFound)
	})

	err = http.ListenAndServe(`:8083`, r)
	if err != http.ErrServerClosed {
		logrus.WithError(err).Error(`Server died unexpectedly`)
	} else {
		logrus.Info(`server shutting down`)
	}
}
