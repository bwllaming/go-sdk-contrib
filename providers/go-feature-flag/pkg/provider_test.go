package gofeatureflag_test

import (
	"bytes"
	"context"
	"fmt"
	gofeatureflag "github.com/open-feature/go-sdk-contrib/providers/go-feature-flag/pkg"
	of "github.com/open-feature/go-sdk/pkg/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type mockClient struct{}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	mockPath := "../testutils/mock_responses/%s.json"
	flagName := strings.Replace(strings.Replace(req.URL.Path, "/v1/feature/", "", -1), "/eval", "", -1)

	if flagName == "unauthorized" {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	}

	content, err := os.ReadFile(fmt.Sprintf(mockPath, flagName))
	if err != nil {
		content, _ = os.ReadFile(fmt.Sprintf(mockPath, "flag_not_found"))
	}
	body := io.NopCloser(bytes.NewReader(content))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}, nil
}

func defaultEvaluationCtx() of.EvaluationContext {
	return of.NewEvaluationContext(
		"d45e303a-38c2-11ed-a261-0242ac120002",
		map[string]interface{}{
			"email":        "john.doe@gofeatureflag.org",
			"firstname":    "john",
			"lastname":     "doe",
			"anonymous":    false,
			"professional": true,
			"rate":         3.14,
			"age":          30,
			"admin":        true,
			"company_info": map[string]interface{}{
				"name": "my_company",
				"size": 120,
			},
			"labels": []string{
				"pro", "beta",
			},
		},
	)
}

func TestProvider_BooleanEvaluation(t *testing.T) {
	type args struct {
		flag         string
		defaultValue bool
		evalCtx      of.EvaluationContext
	}
	tests := []struct {
		name string
		args args
		want of.BooleanEvaluationDetails
	}{
		{
			name: "unauthorized flag",
			args: args{
				flag:         "unauthorized",
				defaultValue: false,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.BooleanEvaluationDetails{
				Value: false,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "unauthorized",
					FlagType: of.Boolean,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.GeneralCode,
						ErrorMessage: "invalid token used to contact GO Feature Flag relay proxy instance",
					},
				},
			},
		},
		{
			name: "should resolve a valid boolean flag with TARGETING_MATCH reason",
			args: args{
				flag:         "bool_targeting_match",
				defaultValue: false,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.BooleanEvaluationDetails{
				Value: true,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "bool_targeting_match",
					FlagType: of.Boolean,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "True",
						Reason:       of.TargetingMatchReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should use boolean default value if the flag is disabled",
			args: args{
				flag:         "disabled_bool",
				defaultValue: false,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.BooleanEvaluationDetails{
				Value: false,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "disabled_bool",
					FlagType: of.Boolean,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "SdkDefault",
						Reason:       of.DisabledReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should error if we expect a boolean and got another type",
			args: args{
				flag:         "string_key",
				defaultValue: false,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.BooleanEvaluationDetails{
				Value: false,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "string_key",
					FlagType: of.Boolean,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.TypeMismatchCode,
						ErrorMessage: "unexpected type for flag string_key",
					},
				},
			},
		},
		{
			name: "should error if flag does not exists",
			args: args{
				flag:         "does_not_exists",
				defaultValue: false,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.BooleanEvaluationDetails{
				Value: false,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "does_not_exists",
					FlagType: of.Boolean,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.FlagNotFoundCode,
						ErrorMessage: "flag does_not_exists was not found in GO Feature Flag",
					},
				},
			},
		},
		{
			name: "should return custom reason if returned by relay proxy",
			args: args{
				flag:         "unknown_reason",
				defaultValue: false,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.BooleanEvaluationDetails{
				Value: true,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "unknown_reason",
					FlagType: of.Boolean,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "True",
						Reason:       "CUSTOM_REASON",
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should return error if no targeting key",
			args: args{
				flag:         "bool_targeting_match",
				defaultValue: false,
				evalCtx:      of.EvaluationContext{},
			},
			want: of.BooleanEvaluationDetails{
				Value: false,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "bool_targeting_match",
					FlagType: of.Boolean,
					ResolutionDetail: of.ResolutionDetail{
						Reason:       of.ErrorReason,
						ErrorCode:    of.TargetingKeyMissingCode,
						ErrorMessage: "no targetingKey provided in the evaluation context",
					},
				},
			},
		},
		{
			name: "should return an error if invalid json body",
			args: args{
				flag:         "invalid_json_body",
				defaultValue: false,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.BooleanEvaluationDetails{
				Value: false,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "invalid_json_body",
					FlagType: of.Boolean,
					ResolutionDetail: of.ResolutionDetail{
						Reason:       of.ErrorReason,
						ErrorCode:    of.ParseErrorCode,
						ErrorMessage: "impossible to parse response for flag invalid_json_body: {\n  \"trackEvents\": true,\n  \"variationType\": \"True\",\n  \"failed\": false,\n  \"version\": \"\",\n  \"reason\": \"TARGETING_MATCH\",\n  \"errorCode\": \"\",\n  \"value\": true\n",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := gofeatureflag.ProviderOptions{
				Endpoint:            "https://gofeatureflag.org/",
				HTTPClient:          &mockClient{},
				GOFeatureFlagConfig: nil,
			}
			provider, err := gofeatureflag.NewProvider(options)
			assert.NoError(t, err)
			of.SetProvider(provider)
			client := of.NewClient("test-app")
			value, err := client.BooleanValueDetails(context.TODO(), tt.args.flag, tt.args.defaultValue, tt.args.evalCtx)

			if tt.want.ErrorCode != "" {
				assert.Error(t, err)
				want := fmt.Sprintf("error code: %s: %s", tt.want.ErrorCode, tt.want.ErrorMessage)
				assert.Equal(t, want, err.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, value)
		})
	}
}

func TestProvider_StringEvaluation(t *testing.T) {
	type args struct {
		flag         string
		defaultValue string
		evalCtx      of.EvaluationContext
	}
	tests := []struct {
		name string
		args args
		want of.StringEvaluationDetails
	}{
		{
			name: "should resolve a valid string flag with TARGETING_MATCH reason",
			args: args{
				flag:         "string_key",
				defaultValue: "default",
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.StringEvaluationDetails{
				Value: "CC0000",
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "string_key",
					FlagType: of.String,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "True",
						Reason:       of.TargetingMatchReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should use string default value if the flag is disabled",
			args: args{
				flag:         "disabled_string",
				defaultValue: "default",
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.StringEvaluationDetails{
				Value: "default",
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "disabled_string",
					FlagType: of.String,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "SdkDefault",
						Reason:       of.DisabledReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should error if we expect a string and got another type",
			args: args{
				flag:         "bool_targeting_match",
				defaultValue: "default",
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.StringEvaluationDetails{
				Value: "default",
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "bool_targeting_match",
					FlagType: of.String,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.TypeMismatchCode,
						ErrorMessage: "unexpected type for flag bool_targeting_match",
					},
				},
			},
		},
		{
			name: "should error if flag does not exists",
			args: args{
				flag:         "does_not_exists",
				defaultValue: "default",
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.StringEvaluationDetails{
				Value: "default",
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "does_not_exists",
					FlagType: of.String,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.FlagNotFoundCode,
						ErrorMessage: "flag does_not_exists was not found in GO Feature Flag",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := gofeatureflag.ProviderOptions{
				Endpoint:            "https://gofeatureflag.org/",
				HTTPClient:          &mockClient{},
				GOFeatureFlagConfig: nil,
			}
			provider, err := gofeatureflag.NewProvider(options)
			assert.NoError(t, err)
			of.SetProvider(provider)
			client := of.NewClient("test-app")
			value, err := client.StringValueDetails(context.TODO(), tt.args.flag, tt.args.defaultValue, tt.args.evalCtx)

			if tt.want.ErrorCode != "" {
				assert.Error(t, err)
				want := fmt.Sprintf("error code: %s: %s", tt.want.ErrorCode, tt.want.ErrorMessage)
				assert.Equal(t, want, err.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, value)
		})
	}
}

func TestProvider_FloatEvaluation(t *testing.T) {
	type args struct {
		flag         string
		defaultValue float64
		evalCtx      of.EvaluationContext
	}
	tests := []struct {
		name string
		args args
		want of.FloatEvaluationDetails
	}{
		{
			name: "should resolve a valid float flag with TARGETING_MATCH reason",
			args: args{
				flag:         "double_key",
				defaultValue: 123.45,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.FloatEvaluationDetails{
				Value: 100.25,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "double_key",
					FlagType: of.Float,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "True",
						Reason:       of.TargetingMatchReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should use float default value if the flag is disabled",
			args: args{
				flag:         "disabled_float",
				defaultValue: 123.45,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.FloatEvaluationDetails{
				Value: 123.45,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "disabled_float",
					FlagType: of.Float,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "SdkDefault",
						Reason:       of.DisabledReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should error if we expect a string and got another type",
			args: args{
				flag:         "bool_targeting_match",
				defaultValue: 123.45,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.FloatEvaluationDetails{
				Value: 123.45,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "bool_targeting_match",
					FlagType: of.Float,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.TypeMismatchCode,
						ErrorMessage: "unexpected type for flag bool_targeting_match",
					},
				},
			},
		},
		{
			name: "should error if flag does not exists",
			args: args{
				flag:         "does_not_exists",
				defaultValue: 123.45,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.FloatEvaluationDetails{
				Value: 123.45,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "does_not_exists",
					FlagType: of.Float,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.FlagNotFoundCode,
						ErrorMessage: "flag does_not_exists was not found in GO Feature Flag",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := gofeatureflag.ProviderOptions{
				Endpoint:            "https://gofeatureflag.org/",
				HTTPClient:          &mockClient{},
				GOFeatureFlagConfig: nil,
			}
			provider, err := gofeatureflag.NewProvider(options)
			assert.NoError(t, err)
			of.SetProvider(provider)
			client := of.NewClient("test-app")
			value, err := client.FloatValueDetails(context.TODO(), tt.args.flag, tt.args.defaultValue, tt.args.evalCtx)

			if tt.want.ErrorCode != "" {
				assert.Error(t, err)
				want := fmt.Sprintf("error code: %s: %s", tt.want.ErrorCode, tt.want.ErrorMessage)
				assert.Equal(t, want, err.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, value)
		})
	}
}

func TestProvider_IntEvaluation(t *testing.T) {
	type args struct {
		flag         string
		defaultValue int64
		evalCtx      of.EvaluationContext
	}
	tests := []struct {
		name string
		args args
		want of.IntEvaluationDetails
	}{
		{
			name: "should resolve a valid float flag with TARGETING_MATCH reason",
			args: args{
				flag:         "integer_key",
				defaultValue: 123,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.IntEvaluationDetails{
				Value: 100,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "integer_key",
					FlagType: of.Int,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "True",
						Reason:       of.TargetingMatchReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should use float default value if the flag is disabled",
			args: args{
				flag:         "disabled_int",
				defaultValue: 123,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.IntEvaluationDetails{
				Value: 123,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "disabled_int",
					FlagType: of.Int,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "SdkDefault",
						Reason:       of.DisabledReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should error if we expect a string and got another type",
			args: args{
				flag:         "bool_targeting_match",
				defaultValue: 123,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.IntEvaluationDetails{
				Value: 123,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "bool_targeting_match",
					FlagType: of.Int,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.TypeMismatchCode,
						ErrorMessage: "unexpected type for flag bool_targeting_match",
					},
				},
			},
		},
		{
			name: "should error if flag does not exists",
			args: args{
				flag:         "does_not_exists",
				defaultValue: 123,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.IntEvaluationDetails{
				Value: 123,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "does_not_exists",
					FlagType: of.Int,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.FlagNotFoundCode,
						ErrorMessage: "flag does_not_exists was not found in GO Feature Flag",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := gofeatureflag.ProviderOptions{
				Endpoint:            "https://gofeatureflag.org/",
				HTTPClient:          &mockClient{},
				GOFeatureFlagConfig: nil,
			}
			provider, err := gofeatureflag.NewProvider(options)
			assert.NoError(t, err)
			of.SetProvider(provider)
			client := of.NewClient("test-app")
			value, err := client.IntValueDetails(context.TODO(), tt.args.flag, tt.args.defaultValue, tt.args.evalCtx)

			if tt.want.ErrorCode != "" {
				assert.Error(t, err)
				want := fmt.Sprintf("error code: %s: %s", tt.want.ErrorCode, tt.want.ErrorMessage)
				assert.Equal(t, want, err.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, value)
		})
	}
}

func TestProvider_ObjectEvaluation(t *testing.T) {
	type args struct {
		flag         string
		defaultValue interface{}
		evalCtx      of.EvaluationContext
	}
	tests := []struct {
		name string
		args args
		want of.InterfaceEvaluationDetails
	}{
		{
			name: "should resolve a valid interface flag with TARGETING_MATCH reason",
			args: args{
				flag:         "object_key",
				defaultValue: nil,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.InterfaceEvaluationDetails{
				Value: map[string]interface{}{
					"test":  "test1",
					"test2": false,
					"test3": 123.3,
					"test4": float64(1),
					"test5": nil,
				},
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "object_key",
					FlagType: of.Object,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "True",
						Reason:       of.TargetingMatchReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should use interface default value if the flag is disabled",
			args: args{
				flag:         "disabled_int",
				defaultValue: nil,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.InterfaceEvaluationDetails{
				Value: nil,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "disabled_int",
					FlagType: of.Object,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "SdkDefault",
						Reason:       of.DisabledReason,
						ErrorCode:    "",
						ErrorMessage: "",
					},
				},
			},
		},
		{
			name: "should error if flag does not exists",
			args: args{
				flag:         "does_not_exists",
				defaultValue: nil,
				evalCtx:      defaultEvaluationCtx(),
			},
			want: of.InterfaceEvaluationDetails{
				Value: nil,
				EvaluationDetails: of.EvaluationDetails{
					FlagKey:  "does_not_exists",
					FlagType: of.Object,
					ResolutionDetail: of.ResolutionDetail{
						Variant:      "",
						Reason:       of.ErrorReason,
						ErrorCode:    of.FlagNotFoundCode,
						ErrorMessage: "flag does_not_exists was not found in GO Feature Flag",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := gofeatureflag.ProviderOptions{
				Endpoint:            "https://gofeatureflag.org/",
				HTTPClient:          &mockClient{},
				GOFeatureFlagConfig: nil,
			}
			provider, err := gofeatureflag.NewProvider(options)
			assert.NoError(t, err)
			of.SetProvider(provider)
			client := of.NewClient("test-app")
			value, err := client.ObjectValueDetails(context.TODO(), tt.args.flag, tt.args.defaultValue, tt.args.evalCtx)

			if tt.want.ErrorCode != "" {
				require.Error(t, err)
				want := fmt.Sprintf("error code: %s: %s", tt.want.ErrorCode, tt.want.ErrorMessage)
				assert.Equal(t, want, err.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, value)
		})
	}
}
