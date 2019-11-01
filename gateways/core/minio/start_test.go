/*
Copyright 2018 BlackRock, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package minio

import (
	"github.com/ghodss/yaml"
	"testing"

	"github.com/argoproj/argo-events/common"
	"github.com/argoproj/argo-events/gateways"
	apicommon "github.com/argoproj/argo-events/pkg/apis/common"
	"github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListeEvents(t *testing.T) {
	convey.Convey("Given an event source, listen to events", t, func() {
		listener := &EventListener{
			k8sClient: fake.NewSimpleClientset(),
			logger:    common.NewArgoEventsLogger(),
			namespace: "fake",
		}
		secret, err := listener.k8sClient.CoreV1().Secrets(listener.namespace).Create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "artifacts-minio",
				Namespace: listener.namespace,
			},
			Data: map[string][]byte{
				"accesskey": []byte("access"),
				"secretkey": []byte("secret"),
			},
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(secret, convey.ShouldNotBeNil)

		dataCh := make(chan []byte)
		errorCh := make(chan error)
		doneCh := make(chan struct{}, 1)
		errCh2 := make(chan error)

		go func() {
			err := <-errorCh
			errCh2 <- err
		}()

		var minioEventSource *apicommon.S3Artifact
		yaml.Unmarshal(secret.Data)

		ps, err := parseEventSource(es)
		convey.So(err, convey.ShouldBeNil)
		listener.listenEvents(ps.(*apicommon.S3Artifact), &gateways.EventSource{
			Id:   "1234",
			Data: es,
			Name: "fake",
		}, dataCh, errorCh, doneCh)

		err = <-errCh2
		convey.So(err, convey.ShouldNotBeNil)
	})
}