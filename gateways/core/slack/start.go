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

package slack

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/argoproj/argo-events/common"
	"github.com/argoproj/argo-events/gateways"
	gwcommon "github.com/argoproj/argo-events/gateways/common"
	"github.com/argoproj/argo-events/pkg/apis/eventsources/v1alpha1"
	"github.com/argoproj/argo-events/store"
	"github.com/ghodss/yaml"
	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"github.com/pkg/errors"
)

var (
	helper = gwcommon.NewWebhookController()
)

func init() {
	go gwcommon.InitializeRouteChannels(helper)
}

func (rc *RouteConfig) GetRoute() *gwcommon.Route {
	return rc.route
}

// HandleRoute handles new route
func (rc *RouteConfig) HandleRoute(writer http.ResponseWriter, request *http.Request) {
	r := rc.route

	log := r.Logger.WithFields(
		map[string]interface{}{
			common.LabelEventSource: r.EventSource.Name,
			common.LabelEndpoint:    r.Webhook.Endpoint,
			common.LabelPort:        r.Webhook.Port,
			common.LabelHTTPMethod:  r.Webhook.Method,
		})

	log.Info("request received")

	if !helper.ActiveEndpoints[r.Webhook.Endpoint].Active {
		log.Warn("endpoint is not active")
		common.SendErrorResponse(writer, "")
		return
	}

	err := rc.verifyRequest(request)
	if err != nil {
		log.WithError(err).Error("Failed validating request")
		common.SendInternalErrorResponse(writer, "")
		return
	}

	var data []byte
	// Interactive element actions are always
	// sent as application/x-www-form-urlencoded
	// If request was generated by an interactive element, it will be a POST form
	if len(request.Header["Content-Type"]) > 0 && request.Header["Content-Type"][0] == "application/x-www-form-urlencoded" {
		data, err = rc.handleInteraction(request)
		if err != nil {
			log.WithError(err).Error("Failed processing interaction")
			common.SendInternalErrorResponse(writer, "")
			return
		}
	} else {
		// If there's no payload in the post body, this is likely an
		// Event API request. Parse and process if valid.
		var response []byte
		data, response, err = rc.handleEvent(request)
		if err != nil {
			log.WithError(err).Error("Failed processing event")
			common.SendInternalErrorResponse(writer, "")
			return
		}
		if response != nil {
			writer.Header().Set("Content-Type", "text")
			if _, err := writer.Write(response); err != nil {
				log.WithError(err).Error("failed to write the response for url verification")
				// don't return, we want to keep this running to give user chance to retry
			}
		}
	}

	if data != nil {
		helper.ActiveEndpoints[rc.route.Webhook.Endpoint].DataCh <- data
	}

	log.Info("request successfully processed")
	common.SendSuccessResponse(writer, "")
}

func (rc *RouteConfig) handleEvent(request *http.Request) ([]byte, []byte, error) {
	var err error
	var response []byte
	var data []byte
	body, err := getRequestBody(request)
	if err != nil {
		return data, response, errors.Wrap(err, "failed to fetch request body")
	}

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: rc.token}))
	if err != nil {
		return data, response, errors.Wrap(err, "failed to extract event")
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err = json.Unmarshal([]byte(body), &r)
		if err != nil {
			return data, response, errors.Wrap(err, "failed to verify the challenge")
		}
		response = []byte(r.Challenge)
	}

	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		data, err = json.Marshal(eventsAPIEvent.InnerEvent.Data)
		if err != nil {
			return data, response, errors.Wrap(err, "failed to marshal event data")
		}
	}

	return data, response, nil
}

func (rc *RouteConfig) handleInteraction(request *http.Request) ([]byte, error) {
	var err error
	err = request.ParseForm()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse post body")
	}

	payload := request.PostForm.Get("payload")
	ie := &slack.InteractionCallback{}
	err = json.Unmarshal([]byte(payload), ie)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse interaction event")
	}

	data, err := json.Marshal(ie)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal action data")
	}

	return data, nil
}

func getRequestBody(request *http.Request) ([]byte, error) {
	// Read request payload
	body, err := ioutil.ReadAll(request.Body)
	// Reset request.Body ReadCloser to prevent side-effect if re-read
	request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse request body")
	}
	return body, nil
}

// If a signing secret is provided, validate the request against the
// X-Slack-Signature header value.
// The signature is a hash generated as per Slack documentation at:
// https://api.slack.com/docs/verifying-requests-from-slack
func (rc *RouteConfig) verifyRequest(request *http.Request) error {
	signingSecret := rc.signingSecret
	if len(signingSecret) > 0 {
		sv, err := slack.NewSecretsVerifier(request.Header, signingSecret)
		if err != nil {
			return errors.Wrap(err, "cannot create secrets verifier")
		}

		// Read the request body
		body, err := getRequestBody(request)
		if err != nil {
			return err
		}

		_, err = sv.Write([]byte(string(body)))
		if err != nil {
			return errors.Wrap(err, "error writing body: cannot verify signature")
		}

		err = sv.Ensure()
		if err != nil {
			return errors.Wrap(err, "signature validation failed")
		}
	}
	return nil
}

func (rc *RouteConfig) PostStart() error {
	return nil
}

func (rc *RouteConfig) PostStop() error {
	return nil
}

// StartEventSource starts a event source
func (listener *EventListener) StartEventSource(eventSource *gateways.EventSource, eventStream gateways.Eventing_StartEventSourceServer) error {
	defer gateways.Recover(eventSource.Name)

	log := listener.Logger.WithField(common.LabelEventSource, eventSource.Name)
	log.Infoln("operating on event source")

	var slackEventSource *v1alpha1.SlackEventSource
	if err := yaml.Unmarshal(eventSource.Value, &slackEventSource); err != nil {
		log.WithError(err).Errorln("failed to parse the event source")
		return err
	}

	token, err := store.GetSecrets(listener.Clientset, listener.Namespace, slackEventSource.Token.Name, slackEventSource.Token.Key)
	if err != nil {
		log.WithError(err).Error("failed to retrieve token")
		return err
	}

	signingSecret, err := store.GetSecrets(listener.Clientset, listener.Namespace, slackEventSource.SigningSecret.Name, slackEventSource.SigningSecret.Key)
	if err != nil {
		log.WithError(err).Warn("Signing secret not provided. Signature not validated.")
		signingSecret = ""
	}

	return gwcommon.ProcessRoute(&RouteConfig{
		route: &gwcommon.Route{
			Logger:      listener.Logger,
			StartCh:     make(chan struct{}),
			Webhook:     slackEventSource.WebHook,
			EventSource: eventSource,
		},
		token:         token,
		signingSecret: signingSecret,
		clientset:     listener.Clientset,
		namespace:     listener.Namespace,
		eventSource:   slackEventSource,
	}, helper, eventStream)
}