package vshnredis

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

// test getRedisURL function
func Test_getRedisURL(t *testing.T) {
	tests := []struct {
		name string
		s    *v1.Secret
		want string
	}{
		{
			name: "getRedisURL",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte("redis"),
					"REDIS_USERNAME": []byte("user"),
					"REDIS_PASSWORD": []byte("pass"),
					"REDIS_PORT":     []byte("6379"),
				},
			},
			want: "rediss://user:pass@redis:6379",
		},
		{
			name: "getRedisURLWithEmptyPassword",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte("redis"),
					"REDIS_USERNAME": []byte("user"),
					"REDIS_PASSWORD": []byte(""),
					"REDIS_PORT":     []byte("6379"),
				},
			},
			want: "",
		},
		{
			name: "getRedisURLWithEmptyUsername",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte("redis"),
					"REDIS_USERNAME": []byte(""),
					"REDIS_PASSWORD": []byte("pass"),
					"REDIS_PORT":     []byte("6379"),
				},
			},
			want: "",
		},
		{
			name: "getRedisURLWithEmptyPort",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte("redis"),
					"REDIS_USERNAME": []byte("user"),
					"REDIS_PASSWORD": []byte("pass"),
					"REDIS_PORT":     []byte(""),
				},
			},
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHost",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte(""),
					"REDIS_USERNAME": []byte("user"),
					"REDIS_PASSWORD": []byte("pass"),
					"REDIS_PORT":     []byte("6379"),
				},
			},
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPort",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte(""),
					"REDIS_USERNAME": []byte("user"),
					"REDIS_PASSWORD": []byte("pass"),
					"REDIS_PORT":     []byte(""),
				},
			},
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPortAndUsername",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte(""),
					"REDIS_USERNAME": []byte(""),
					"REDIS_PASSWORD": []byte("pass"),
					"REDIS_PORT":     []byte(""),
				},
			},
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPortAndUsernameAndPassword",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte(""),
					"REDIS_USERNAME": []byte(""),
					"REDIS_PASSWORD": []byte(""),
					"REDIS_PORT":     []byte(""),
				},
			},
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPortAndUsernameAndPasswordAndExtra",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte(""),
					"REDIS_USERNAME": []byte(""),
					"REDIS_PASSWORD": []byte(""),
					"REDIS_PORT":     []byte(""),
					"EXTRA":          []byte("extra"),
				},
			},
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPortAndUsernameAndPasswordAndExtraAndRedisUrl",
			s: &v1.Secret{
				Data: map[string][]byte{
					"REDIS_HOST":     []byte(""),
					"REDIS_USERNAME": []byte(""),
					"REDIS_PASSWORD": []byte(""),
					"REDIS_PORT":     []byte(""),
					"EXTRA":          []byte("extra"),
					"REDIS_URL":      []byte("redis://user:pass@redis:6379"),
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		ret := getRedisURL(tt.s)
		if ret != tt.want {
			t.Errorf("getRedisURL() = %v, want %v", ret, tt.want)
		}

	}
}
