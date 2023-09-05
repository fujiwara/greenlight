package greenlight_test

import (
	"testing"

	"github.com/fujiwara/greenlight"
)

func TestNewExpectCodeFunc(t *testing.T) {
	tests := []struct {
		codes     string
		testCases []struct {
			code       int
			expectTrue bool
		}
		expectErr bool
	}{
		{
			codes: "200,201,300-399",
			testCases: []struct {
				code       int
				expectTrue bool
			}{
				{200, true},
				{201, true},
				{300, true},
				{399, true},
				{400, false},
				{199, false},
			},
			expectErr: false,
		},
		{
			codes: " 200 , 201 , 300 - 399 ",
			testCases: []struct {
				code       int
				expectTrue bool
			}{
				{200, true},
				{201, true},
				{300, true},
				{399, true},
				{400, false},
				{199, false},
			},
			expectErr: false,
		},
		{
			codes:     "invalid",
			testCases: nil,
			expectErr: true,
		},
	}

	for i, test := range tests {
		expectCodeFunc, err := greenlight.NewExpectCodeFunc(test.codes)
		if (err != nil) != test.expectErr {
			t.Errorf("Test %d: expected error: %v, got: %v", i, test.expectErr, err)
			continue
		}

		if err != nil {
			continue
		}

		for _, testCase := range test.testCases {
			if got := expectCodeFunc(testCase.code); got != testCase.expectTrue {
				t.Errorf("Test %d: for code %d, expected %v but got %v", i, testCase.code, testCase.expectTrue, got)
			}
		}
	}
}
