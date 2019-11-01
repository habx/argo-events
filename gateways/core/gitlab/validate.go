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

package gitlab

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-events/gateways/common/webhook"

	"github.com/argoproj/argo-events/gateways"
	gwcommon "github.com/argoproj/argo-events/gateways/common"
	"github.com/argoproj/argo-events/pkg/apis/eventsources/v1alpha1"
	"github.com/ghodss/yaml"
)

// ValidateEventSource validates gitlab gateway event source
func (listener *GitlabEventListener) ValidateEventSource(ctx context.Context, eventSource *gateways.EventSource) (*gateways.ValidEventSource, error) {
	var gitlabEventSource *v1alpha1.GitlabEventSource
	if err := yaml.Unmarshal(eventSource.Value, &gitlabEventSource); err != nil {
		listener.Logger.WithError(err).Error("failed to parse the event source")
		return &gateways.ValidEventSource{
			IsValid: false,
			Reason:  err.Error(),
		}, nil
	}

	if err := validateGitlabEventSource(gitlabEventSource); err != nil {
		listener.Logger.WithError(err).Error("failed to validate gitlab event source")
		return &gateways.ValidEventSource{
			IsValid: false,
			Reason:  err.Error(),
		}, nil
	}

	return &gateways.ValidEventSource{
		IsValid: true,
	}, nil
}

// validateGitlabEventSource validates a gitlab event source
func validateGitlabEventSource(eventSource *v1alpha1.GitlabEventSource) error {
	if eventSource == nil {
		return gwcommon.ErrNilEventSource
	}
	if eventSource.ProjectId == "" {
		return fmt.Errorf("project id can't be empty")
	}
	if eventSource.Event == "" {
		return fmt.Errorf("event type can't be empty")
	}
	if eventSource.GitlabBaseURL == "" {
		return fmt.Errorf("gitlab base url can't be empty")
	}
	if eventSource.AccessToken == nil {
		return fmt.Errorf("access token can't be nil")
	}
	return webhook.ValidateWebhookContext(eventSource.Webhook)
}