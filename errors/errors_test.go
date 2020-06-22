// Copyright 2019 Google LLC
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

package errors

import (
	"testing"
)

func TestDesc(t *testing.T) {
	for _, tt := range []struct {
		in  UpdateError
		out string
	}{
		{WU_E_ITEMNOTFOUND, `The key for the item queried could not be found.`},
		{WU_E_NO_UPDATE, `There are no updates.`},
		{WU_E_PT_HTTP_STATUS_SERVER_ERROR, `Same as HTTP status 500 – An error internal to the server prevented fulfilling the request.`},
		{255, `Unknown error: 0xFF`},
	} {
		o := tt.in.ErrorDesc()
		if o != tt.out {
			t.Errorf("got %q, want %q", o, tt.out)
		}
	}
}

func TestName(t *testing.T) {
	for _, tt := range []struct {
		in  UpdateError
		out string
	}{
		{WU_E_ITEMNOTFOUND, `WU_E_ITEMNOTFOUND`},
		{WU_E_NO_UPDATE, `WU_E_NO_UPDATE`},
		{WU_E_PT_HTTP_STATUS_SERVER_ERROR, `WU_E_PT_HTTP_STATUS_SERVER_ERROR`},
		{255, ``},
	} {
		o := tt.in.ErrorName()
		if o != tt.out {
			t.Errorf("got %q, want %q", o, tt.out)
		}
	}
}

func TestString(t *testing.T) {
	for _, tt := range []struct {
		in  UpdateError
		out string
	}{
		{WU_E_ITEMNOTFOUND, `[WU_E_ITEMNOTFOUND] The key for the item queried could not be found.`},
		{WU_E_NO_UPDATE, `[WU_E_NO_UPDATE] There are no updates.`},
		{WU_E_PT_HTTP_STATUS_SERVER_ERROR, `[WU_E_PT_HTTP_STATUS_SERVER_ERROR] Same as HTTP status 500 – An error internal to the server prevented fulfilling the request.`},
		{255, `[] Unknown error: 0xFF`},
	} {
		o := tt.in.String()
		if tt.in.String() != tt.out {
			t.Errorf("got %q, want %q", o, tt.out)
		}
	}
}
