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
package v1alpha1

import (
	"github.com/pkg/errors"
)

// ValidateEventSource validates a generic event source
func ValidateEventSource(eventSource *EventSource) error {
	if eventSource == nil {
		return errors.New("event source can't be nil")
	}
	if eventSource.Spec == nil {
		return errors.New("event source specification can't be nil")
	}
	if eventSource.Spec.Version == "" {
		return errors.New("event source version can't be empty")
	}
	return nil
}
