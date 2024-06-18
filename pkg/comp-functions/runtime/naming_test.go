package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceRuntime_escapeK8sNames(t *testing.T) {

	type args struct {
		name       string
		allowColon bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "GivenInvalidString_ThenExpectValidString",
			args: args{
				name: "Hello_World",
			},
			want: "hello-world",
		},
		{
			name: "GivenValidString_ThenExpectValidString",
			args: args{
				name: "hello-world",
			},
			want: "hello-world",
		},
		{
			name: "GivenStringStartingWithUnderscore_ThenExpectValidString",
			args: args{
				name: "_Hello_World",
			},
			want: "hello-world",
		},
		{
			name: "GivenMoreInvalidCharacters_ThenExpectValidString",
			args: args{
				name: "+*ç%&/()=?b+*ç%&/()=?",
			},
			want: "b",
		},
		{
			name: "GivenVerylongName_ThenExpectValidString",
			args: args{
				name: "AVeryLongStringWhichShouldBetrimmedToItaSensiblesize1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012",
			},
			want: "averylongstringwhichshouldbetrimmedtoitasensiblesize1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456-fdbe",
		},
		{
			name: "GivenNameWithColon_ThenExpectValisString",
			args: args{
				name:       "test:colon",
				allowColon: true,
			},
			want: "test:colon",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapeDNS1123(tt.args.name, tt.args.allowColon)
			assert.Equal(t, tt.want, got)
		})
	}
}
