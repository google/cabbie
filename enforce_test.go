// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	testData = "testdata/"
)

func TestEnforcements(t *testing.T) {
	tests := []struct {
		in      string
		want    enforcement
		wantErr error
	}{
		{"required.json",
			enforcement{Required: []string{"4018073", "67891011"}},
			nil,
		},
		{"invalid.json",
			enforcement{},
			errParsing,
		},
		{"missing.json",
			enforcement{},
			errInvalidFile,
		},
		{"wrong-filetype.txt",
			enforcement{},
			errFileType,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := enforcements(filepath.Join(testData, tt.in))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("enforcements(%s) returned unexpected diff (-want +got):\n%s", tt.in, diff)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("enforcements(%s) returned unexpected error %v", tt.in, err)
			}
		})
	}
}
