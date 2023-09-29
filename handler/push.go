//nolint:goheader

// Copyright 2014 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/route"

	dto "github.com/prometheus/client_model/go"

	"github.com/jasonyangshadow/apptheus/internal/util"
	"github.com/jasonyangshadow/apptheus/storage"
)

// Push returns an http.Handler which accepts samples over HTTP and stores them
// in the MetricStore. If replace is true, all metrics for the job and instance
// given by the request are deleted before new ones are stored. If check is
// true, the pushed metrics are immediately checked for consistency (with
// existing metrics and themselves), and an inconsistent push is rejected with
// http.StatusBadRequest.
//
// The returned handler is already instrumented for Prometheus.
func Push(
	ms storage.MetricStore,
	replace, check, jobBase64Encoded bool,
	logger log.Logger,
) func(http.ResponseWriter, *http.Request) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		job := route.Param(r.Context(), "job")
		if jobBase64Encoded {
			var err error
			if job, err = util.DecodeBase64(job); err != nil {
				http.Error(w, fmt.Sprintf("invalid base64 encoding in job name %q: %v", job, err), http.StatusBadRequest)
				level.Debug(logger).Log("msg", "invalid base64 encoding in job name", "job", job, "err", err.Error())
				return
			}
		}
		labelsString := route.Param(r.Context(), "labels")
		labels, err := util.SplitLabels(labelsString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			level.Debug(logger).Log("msg", "failed to parse URL", "url", labelsString, "err", err.Error())
			return
		}
		if job == "" {
			http.Error(w, "job name is required", http.StatusBadRequest)
			level.Debug(logger).Log("msg", "job name is required")
			return
		}
		labels["job"] = job

		var metricFamilies map[string]*dto.MetricFamily
		ctMediatype, ctParams, ctErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if ctErr == nil && ctMediatype == "application/vnd.google.protobuf" &&
			ctParams["encoding"] == "delimited" &&
			ctParams["proto"] == "io.prometheus.client.MetricFamily" {
			metricFamilies = map[string]*dto.MetricFamily{}
			for {
				mf := &dto.MetricFamily{}
				if _, err = pbutil.ReadDelimited(r.Body, mf); err != nil {
					if errors.Is(err, io.EOF) {
						err = nil
					}
					break
				}
				metricFamilies[mf.GetName()] = mf
			}
		} else {
			// We could do further content-type checks here, but the
			// fallback for now will anyway be the text format
			// version 0.0.4, so just go for it and see if it works.
			var parser expfmt.TextParser
			metricFamilies, err = parser.TextToMetricFamilies(r.Body)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			level.Debug(logger).Log("msg", "failed to parse text", "source", r.RemoteAddr, "err", err.Error())
			return
		}
		now := time.Now()
		if !check {
			ms.SubmitWriteRequest(storage.WriteRequest{
				Labels:         labels,
				Timestamp:      now,
				MetricFamilies: metricFamilies,
				Replace:        replace,
			})
			w.WriteHeader(http.StatusAccepted)
			return
		}
		errCh := make(chan error, 1)
		errReceived := false
		ms.SubmitWriteRequest(storage.WriteRequest{
			Labels:         labels,
			Timestamp:      now,
			MetricFamilies: metricFamilies,
			Replace:        replace,
			Done:           errCh,
		})
		for err := range errCh {
			// Send only first error via HTTP, but log all of them.
			// TODO(beorn): Consider sending all errors once we
			// have a use case. (Currently, at most one error is
			// produced.)
			if !errReceived {
				http.Error(
					w,
					fmt.Sprintf("pushed metrics are invalid or inconsistent with existing metrics: %v", err),
					http.StatusBadRequest,
				)
			}
			level.Error(logger).Log(
				"msg", "pushed metrics are invalid or inconsistent with existing metrics",
				"method", r.Method,
				"source", r.RemoteAddr,
				"err", err.Error(),
			)
			errReceived = true
		}
	})

	instrumentedHandler := promhttp.InstrumentHandlerRequestSize(
		httpPushSize, promhttp.InstrumentHandlerDuration(
			httpPushDuration, InstrumentWithCounter("push", handler),
		))

	return func(w http.ResponseWriter, r *http.Request) {
		instrumentedHandler.ServeHTTP(w, r)
	}
}
