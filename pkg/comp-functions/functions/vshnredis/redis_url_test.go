package vshnredis

/* import (
	"testing"
)

// test getRedisURL function
func Test_getRedisURL(t *testing.T) {
	tests := []struct {
		name string
		s    map[string][]byte
		host string
		port string
		user string
		want string
	}{
		{
			name: "getRedisURL",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte("pass"),
			},
			host: "redis",
			port: "6379",
			user: "user",
			want: "rediss://user:pass@redis:6379",
		},
		{
			name: "getRedisURLWithEmptyPassword",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte(""),
			},

			host: "redis",
			port: "6379",
			user: "user",
			want: "",
		},
		{
			name: "getRedisURLWithEmptyUsername",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte("pass"),
			},

			host: "redis",
			port: "6379",
			user: "",
			want: "",
		},
		{
			name: "getRedisURLWithEmptyPort",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte("pass"),
			},

			host: "redis",
			port: "",
			user: "user",
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHost",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte("pass"),
				"REDIS_PORT":     []byte("6379"),
			},

			host: "",
			port: "6379",
			user: "user",
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPort",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte("pass"),
			},

			host: "",
			port: "",
			user: "user",
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPortAndUsername",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte("pass"),
			},

			host: "",
			port: "",
			user: "",
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPortAndUsernameAndPassword",
			s:    map[string][]byte{},

			host: "",
			port: "",
			user: "",
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPortAndUsernameAndPasswordAndExtra",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte(""),
				"EXTRA":          []byte("extra"),
			},

			host: "",
			port: "",
			user: "",
			want: "",
		},
		{
			name: "getRedisURLWithEmptyHostAndPortAndUsernameAndPasswordAndExtraAndRedisUrl",
			s: map[string][]byte{
				"REDIS_PASSWORD": []byte(""),
				"EXTRA":          []byte("extra"),
				"REDIS_URL":      []byte("redis://user:pass@redis:6379"),
			},

			host: "",
			port: "",
			user: "",
			want: "",
		},
	}
	for _, tt := range tests {
		ret := getRedisURL(tt.s, tt.host, tt.port, tt.user)
		if ret != tt.want {
			t.Errorf("getRedisURL() = %v, want %v", ret, tt.want)
		}

	}
}
*/
