package vshnrediscontroller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

/*
BELOW CERTIFICATES IMITATES PRODUCTION CERTIFICATES BUT THEY ARE NOT USED ANYWHERE
THEY ARE TAKEN FROM TEST INSTANCE IN LOCAL KIND CLUSTER


Those certificates are necessary in valid form as I use library that checks them and ensures they are correct and matching
*/

var ca_crt = []byte(`-----BEGIN CERTIFICATE-----
MIIFHzCCAwegAwIBAgIRALCwTKkL658B2Yo7qdQzEh8wDQYJKoZIhvcNAQELBQAw
GTEXMBUGA1UEChMOdnNobi1hcHBjYXQtY2EwHhcNMjMwODE4MTQzMjM5WhcNMzMw
ODE1MTQzMjM5WjAZMRcwFQYDVQQKEw52c2huLWFwcGNhdC1jYTCCAiIwDQYJKoZI
hvcNAQEBBQADggIPADCCAgoCggIBALz8qKjzlMwIprR2MdSJsqrXqYyB+QoZJ+yP
dEIgvUuM9MG6xHF3wsr0uhnNetOaKwvwo/WgKQutgWPrxodT8xc/3cryG/FVbMvT
/r1VDZ8TGrjBp/P1nylLYKyrLGYfZGKorAiuXSXJXB2L6MuQKlpOsZt8dwVwdI4v
2C/gg5SsdVeHFGqVZN8Lzd5PA65SWr4NVMkTfuYV54A5A09Hhe3Q60EyI8MUVYud
9hCeFetDICZUPktizLgm3EwoXlOmeUEh0TpznoPWGcUxCBOInm3q4lXfXb+fYOJD
KvQj0xPrmCnZlzHEiwlxad/JkNYOg93Iosy8Me8fqgTBagYjF8xNII1yZg1jFeRe
euPC1yQB23uR5GNOzlf3DxsRfJ0WWuSokE1AwJd9dj+G4/RTIqBqoWr6IEQaSJlv
k4Q9yz9lNzQ+ws8TRZqNk41TwbcVGX6X+fykl4nG/mcm+HM/acxsrnlcqyR+Dxc7
1B0px5N1c174io0jIqd9ml6Ua4GNJnLyEiPdki2Bvr6JcPH9rnEIVLA4PPSy5IQi
ipG8hfRyqd60YvWXOzKo01daYwRlTzOekajGFj/c6nWFBOvwBHOLzohs2vX9YiuL
P6csxhYafS0Ixy7ZoMi6SjsmSRqQp7FdPzzj+B3/MPOlFDynQi2PBEf2XKQeRRY1
01S27EFZAgMBAAGjYjBgMA4GA1UdDwEB/wQEAwICpDAPBgNVHRMBAf8EBTADAQH/
MB0GA1UdDgQWBBRUmY7zJGJcV79fExkZ1KdVNzFtxDAeBgNVHREEFzAVghN2c2hu
LmFwcGNhdC52c2huLmNoMA0GCSqGSIb3DQEBCwUAA4ICAQAnrQW6EOxQ+WrebWeP
aLLP3s0m+mpLdqmTVaFKNpCfrEaknHuRfbJNwG+t0Dc5c7P0M1qHom63oiS9ZIZm
DpORxx4xpG3dfV0+lnsmq3rvEb+WXOo6USawzWEGpm/SXlt1dQIoxaeQ5U9LxPA4
7pOVXI5YlrIh9AozUC0Kx8TVIcphSQsDoNO0zzM0/zoe76N3Imjvdq+1bw6Q2hVu
6NF7vlEpBZMYSZDnK8S95zXV1RrjQR1Yqwoefwgrl2xhGLM6ncEzapYcnD0FONx5
22OmRHsQZzb1n5CBXja53ZeHttdNw6Emakd8lZogB7vJIANi9SwJKrMjqNQq8kyA
Swrgsl8UzcNB3dovNGEuwEhhB4zR0yKrdsKP2wRCIUExKWnTDeRxPjUHB9HL4h5v
97YqsoXl+HihZypJVKyKzZpmTBewyOmRSMqKxBXb1lqi2g8Co9bH2vdk3Nb4HxA4
1JgE2wk6uEqaud3VBl5jpl8TosWkzR5IwvBUVj2pWS309Om9HECBiFT7C3+Dw2mG
Kqg7MEzQyYVxmAeksUO8cG06LzjmkIaAMn9Jgis6KtJz/Tu82T5n3PKu65kPq5P/
5JoLf4RSHfklw1C94UkF85ZG42jSvMvz1oMw39/HaVxHTeRcDKW0HTkjEDh9rKX8
KmFH0ntR93kH9DKCa8E6/7OfmA==
-----END CERTIFICATE-----
`)

var tls_crt = []byte(`-----BEGIN CERTIFICATE-----
MIIFJzCCAw+gAwIBAgIRAPQ4MnN2MtqBMF/+EwrPnUowDQYJKoZIhvcNAQELBQAw
GTEXMBUGA1UEChMOdnNobi1hcHBjYXQtY2EwHhcNMjMwODE4MTQzMjQyWhcNMzMw
ODE1MTQzMjQyWjAdMRswGQYDVQQKExJ2c2huLWFwcGNhdC1jbGllbnQwggIiMA0G
CSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDMRqxR4mv9UaCleT9iOSqK/jDsD4b8
G73kxSSGcuAaby5/06ie+Bo4zQCQcCqWcAbfYGctVWjnrLUiBsyp1zKB9+yLKAiS
2tLMk9noV8IiM3fSKEusgY73bKtbDMwFPUIom7ngIADGL15axxkwORyOjvNbKBqa
a2EnI6KVbch2pLrlXvz5PbVoiHUvgFCHyXzZ1ZsPgBNctl3eUCrQ17jX77zLiuuc
3bxVwmP4nHKJ1Ocj8UEHhUL7cxDswBIPsX/2D+tQfRuVhvWaR7bPeW5x0Ee2Z5Fg
zT/2gqZNLmwr5hr/xzCVqg50k9q7J+aphK0u8jR9irLeuJUxoBiJR3pZR910wEIs
pk8d5HCX5AgE/nKS9Qz9MR/89vQMbz0lwPBL18vZWjdBZKs9j9zBPjm9tEaXqQBW
LV67soAGuC+ojUBKX3oVY74EzonZmod9kTo8F94V0+1DMY7Pb7EHHC3M/EJK15MF
WgOdzrSwDDe4Nx93QwyWN9gtaDdK24leTqK3cFTJqREvFZWHvQOhlp3eUGj1BKUo
p5EGJI0q6SjD6va9vYs8EWI3lgYgxbsPedJdtgkM+rqTVlxD0WKOcNnNqjiBF5Ms
exzUQ8DsJW+6wCugGW/jb6CGFTyvP7qWraV53/MXdVXD8RL4EWmHF3XcLh8gLL1p
H0VZRKMpAsPKIQIDAQABo2YwZDATBgNVHSUEDDAKBggrBgEFBQcDAjAMBgNVHRMB
Af8EAjAAMB8GA1UdIwQYMBaAFFSZjvMkYlxXv18TGRnUp1U3MW3EMB4GA1UdEQQX
MBWCE3ZzaG4uYXBwY2F0LnZzaG4uY2gwDQYJKoZIhvcNAQELBQADggIBAA8MK+Or
5Auw+KNJAIlSKKuqa2XErPulDudM7AD7oFOtEZwLEjSqQGlSN8w8eJ8FUph8TEI2
wRNq0upRK43sGx674MLthK8kW3LZTzlExTtb0aAIo5J0/zUbKS0GltDDCb8eaD9g
xMeDuEu0F47co8IjYDVGATXwOTWKaQjAdYgizmrCDkgDwJllIKoJi9fsxTn8ZpgU
9obPP6FWvNN2z8K7gIEcNqaDBr++D727vkSIuZdRG5wbBPu/eUSi1Mc0QMFTLv6n
qPHNt+EreVBqIs4a7TcznD1kEAqIky5SQ45VsHCOhsQ0tuHR1aZEowUlCpvCytig
tvU3VGfUsMJtr3JgngB7pcn3ltR+uABRBinbcs3Yg+7wCO0hPaYTKv7bN7pkKd9H
BjyNrIEYp8V/ZsFz2+bM8IpMS+pntLOCwLBCKUjxaqI1AXMbNDgzuFYmMLPLYS9i
Uu+C1bvk8Q737bU+uxFqq6CtGV2SMLpRAnpHoGcLhD9JyK153oxumGkmU49d5v3W
MqeOZa3D63ho00Mq4STAD5vyT98xA7D/MAp5a5EWNPPIjxK38M+OQCBCj4qB0bbV
7z8x+NIgg97BpgKwfpzDbM2wy61k6Dt52Yw4V3SPGNTZ/uSoOFOnIQ02FCu2BX+b
uaBKpj84DTTT1R/Wwy6xnvnCK65+8Ty3GOxh
-----END CERTIFICATE-----
`)

var tls_key = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIJKgIBAAKCAgEAzEasUeJr/VGgpXk/Yjkqiv4w7A+G/Bu95MUkhnLgGm8uf9Oo
nvgaOM0AkHAqlnAG32BnLVVo56y1IgbMqdcygffsiygIktrSzJPZ6FfCIjN30ihL
rIGO92yrWwzMBT1CKJu54CAAxi9eWscZMDkcjo7zWygammthJyOilW3IdqS65V78
+T21aIh1L4BQh8l82dWbD4ATXLZd3lAq0Ne41++8y4rrnN28VcJj+JxyidTnI/FB
B4VC+3MQ7MASD7F/9g/rUH0blYb1mke2z3lucdBHtmeRYM0/9oKmTS5sK+Ya/8cw
laoOdJPauyfmqYStLvI0fYqy3riVMaAYiUd6WUfddMBCLKZPHeRwl+QIBP5ykvUM
/TEf/Pb0DG89JcDwS9fL2Vo3QWSrPY/cwT45vbRGl6kAVi1eu7KABrgvqI1ASl96
FWO+BM6J2ZqHfZE6PBfeFdPtQzGOz2+xBxwtzPxCSteTBVoDnc60sAw3uDcfd0MM
ljfYLWg3StuJXk6it3BUyakRLxWVh70DoZad3lBo9QSlKKeRBiSNKukow+r2vb2L
PBFiN5YGIMW7D3nSXbYJDPq6k1ZcQ9FijnDZzao4gReTLHsc1EPA7CVvusAroBlv
42+ghhU8rz+6lq2led/zF3VVw/ES+BFphxd13C4fICy9aR9FWUSjKQLDyiECAwEA
AQKCAgBX0sqjKcVj04RNoCMwB4PS3hYKZ2KRYRvzDw70/s56jvJe4pDMR75+TSTA
9Hha1S8fOkMhqma/s/DsACBdpLeCSSTZbwzAlnOqoKY+zfwY2CfbopdmQw1EMuQ4
8PeGCSA4dTsksJ2klxjpzk91/Bfa8nqN5mAJo8DRIvDzbY+R8qCnnH8GaRFlL8Wx
9mio+GdFACD6OQYaBg21VqIRx60gqWFx4PgLKQmflUOFzz0vJOl6+m4K3bA+iunr
32fkd1ucXpu8rwz87FkLS2m9KWfiZrluInjONXAae3EkHaFD/ln9PZYVwlcUp7yS
WkVA/Fm4pUkL8GWWe6UpALuLyb8/fpJbkfC9rHw8mG/e4haYrfYs4+KMorg5eIN+
NdXwXRrOa8feV8xY/NiUV4eM0IWOdVRw1tw/hdWBYSrIczoDSWvkaKyQHcuDdSkn
rXre1r8RZTuKdJv8Yi0ialS4yWsMEbHg+h4nEjLTKV81RCYDaU3j074+t0YNoqS2
Qpg4S0EqqA7Qf6wAUvPnsoQDp9Ykzwt6fUng18VyBcT1blf86JO2XgcD3A1JNS42
oFOe5gq8VM8uQbsByDwMeTXpyRCkwff+OyetLqTx1GMyku42DFpseh283b4HDEgg
5DhVkG/mvr5qhXvJcLjz4cXFE4VH3KR7FY727HUxq+j9Sg3sAQKCAQEA4uTFyoaW
8p276J7yj6xNOYLHG86D5r3Tp8s4HMEJbaVnb3yzPlERmADjoznjxXM31PIsIwia
ZgpBxEof3HxquiykqUpnC3UhInoLaVDV5D75VHPF9FTW+4blNW/Gf6d5zPI9jwLz
GhBSeWuDXM0cL8u23dnMhHz2EkbFEzHP9dvglqINJF7JOm1BVwSQetYmdrqv/oZQ
9hQMOM+pkwpJkTVxHGER9wgBzrZ1C2GfhcTdS7gAoducNvJra6brjYLhVd0W98TI
HCKNWiIChQEd0ApGPPax0tzP9UPYiFMnR6nIjQhUl4UhOo5owjkHaZCXW5Sdn5zT
QiAw1c/MvDe6mQKCAQEA5nsiowy0sPx1IVtkVJlnMRqVGOwLDP4I4nLocbO7VamR
6/WB/LFDocQ3eGA+zDKLJvjHliFlsxPdmFSiHYgBwTQVwH+ze/DIVghGWsRH3GRK
bdGsno481iTyl1+bVyW/hDqDN2qzLGFwAF35oRFITPLQhEoGantMeyIorZt9vikO
luLsEZgV6ThQhCCXeEsE/uSyjgDptFaniddszxeLEDD0X3r5YB7ZvFdErCe6Jo39
JghVmzjF9SFDDXhyfQxd/JT64td5ibKatRiGkp1XWq8CKSMR9lwpv06iRTQHqyFs
k0UZLpWf1lUgcD8efIU4TY8Xd2AlGMhteXx9ZImIyQKCAQEAr6JdxX9X26jkK5bW
twaupMUqMckz62qoK6ww3HlFPh4aqn+CFMwWbW7Kx7BpM5AT+QAZ0Gi5dCGedT7X
2QpqZ4FlWTKh/4mEw7ZrnPOZDtz8jjYsVw1ReVUbmrjSlEBlFZOyuUCURGm8Hgdu
oWiY2Bq+jI8rNKeyp8Umisw1aeDxwkjhGXVSGas3OA/tc1jQX3n2AHWiuEoeh9+g
KZV+CyyuSUSO7oXBOG2evter75XLo/BkdOaVzybqpmOI1XspyRiizdsC+Fx6xPms
r56EoGVDp69jSZHqXLZPKIAN2PiBqUJ4kO3aIgTY7PfOWBY4RAkP1t1D310h0HDR
0CTniQKCAQEA0HnxCqsjhjPVfya7ygo4XSI+WxynokjmoG5v8ukwOnv3kgewXHG7
S0fBJRFpvCq707SUVChBZYpCltd3DF9Jtwj14/me0C0sCSXS/actmRzedheCnKjs
PoeNJ39Dc8ChS2nh5u6Mw0gflzVp51dKns/D7OVIiGie9YIgaWiMhMV+fN0ly4RV
zW8y5VDVsempyyXynKAWxRjc0sIZmfkhwLOHWBZUG63MJaCKbW5B4z3sDcrcJFtm
NCSyEi0w4guduCrKBQYC2ZrEdaqJj3Ti3xQOUEd4p+8VlAX8obw+c3z4SP3nmUue
GLFHdkChwuB93SnhgAlnhNNGsuz4P0hogQKCAQEAy7v1gMqneXybYJaSiMbcHZWe
ojyLDtnQEiH9GqZNS76CttL+EX8t6vAsonngCMixt5zSZntGCs0T7q8qLfWBdWea
h+mQVrRAtGzShDs487g0E1uffZi3wEPx6oEum9CBKfTiAscazNTBoqeLFF1swL2a
NtKmecRB7/LblNE66/t1s9jRW5VTkxUz9Kkirpsoi4XNsXXtfFq+4DGlC3mwyXbX
9jMLBTSX7xAsTpIg/T/d8RVsOcUFji7clvmG9961NDqAPvW0BUlOHzK4mVbKXPN5
nrfsBwoJpAcOmKDb/zOwtqOiSeZ4ef4DULUpz61vNmKch4N2fDDdcjuHCXo0tA==
-----END RSA PRIVATE KEY-----
`)

func TestVSHNRedis_StartStop(t *testing.T) {
	db := newTestVSHNRedis("bar", "foo", "creds")
	r, manager, client := setupVSHNRedisTest(t,
		db,
		newTestVSHNRedisCred("bar", "creds"),
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}
	pi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "foo",
		Namespace: "bar",
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	require.NoError(t, client.Delete(context.TODO(), db))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNRedis_StartStop_WithFinalizer(t *testing.T) {
	db := newTestVSHNRedis("bar", "foo", "creds")
	db.SetFinalizers([]string{"foobar.vshn.io"})
	r, manager, client := setupVSHNRedisTest(t,
		db,
		newTestVSHNRedisCred("bar", "creds"),
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}
	pi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "foo",
		Namespace: "bar",
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	require.NoError(t, client.Delete(context.TODO(), db))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNRedis_Multi(t *testing.T) {
	dbBar := newTestVSHNRedis("bar", "foo", "creds")
	dbBarer := newTestVSHNRedis("bar", "fooer", "credentials")
	dbBuzz := newTestVSHNRedis("buzz", "foo", "creds")
	r, manager, c := setupVSHNRedisTest(t,
		dbBar,
		newTestVSHNRedisCred("bar", "creds"),
		dbBarer,
		newTestVSHNRedisCred("bar", "credentials"),
		dbBuzz,
		newTestVSHNRedisCred("buzz", "creds"),
	)

	barPi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "foo",
		Namespace: "bar",
	}
	barerPi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "fooer",
		Namespace: "bar",
	}
	buzzPi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "foo",
		Namespace: "buzz",
	}

	_, err := r.Reconcile(context.TODO(), recReq("bar", "foo"))
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), recReq("bar", "fooer"))
	assert.NoError(t, err)

	assert.True(t, manager.probers[getFakeKey(barPi)])
	assert.True(t, manager.probers[getFakeKey(barerPi)])
	assert.False(t, manager.probers[getFakeKey(buzzPi)])

	require.NoError(t, c.Delete(context.TODO(), dbBar))
	_, err = r.Reconcile(context.TODO(), recReq("bar", "foo"))
	assert.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), recReq("buzz", "foo"))
	assert.NoError(t, err)

	assert.False(t, manager.probers[getFakeKey(barPi)])
	assert.True(t, manager.probers[getFakeKey(barerPi)])
	assert.True(t, manager.probers[getFakeKey(buzzPi)])
}

func TestVSHNRedis_Startup_NoCreds_Dont_Probe(t *testing.T) {
	db := newTestVSHNRedis("bar", "foo", "creds")
	db.SetCreationTimestamp(metav1.Now())
	r, manager, _ := setupVSHNRedisTest(t,
		db,
	)
	pi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "foo",
		Namespace: "bar",
	}

	res, err := r.Reconcile(context.TODO(), recReq("bar", "foo"))
	assert.NoError(t, err)
	assert.Greater(t, res.RequeueAfter.Microseconds(), int64(0))

	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNRedis_NoRef_Dont_Probe(t *testing.T) {
	db := newTestVSHNRedis("bar", "foo", "creds")
	db.Spec.WriteConnectionSecretToRef.Name = ""
	r, manager, _ := setupVSHNRedisTest(t,
		db,
	)
	pi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "foo",
		Namespace: "bar",
	}

	_, err := r.Reconcile(context.TODO(), recReq("bar", "foo"))
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNRedis_Started_NoCreds_Probe_Failure(t *testing.T) {
	db := newTestVSHNRedis("bar", "foo", "creds")
	db.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(-1 * time.Hour)})
	r, manager, _ := setupVSHNRedisTest(t,
		db,
	)
	pi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "foo",
		Namespace: "bar",
	}

	res, err := r.Reconcile(context.TODO(), recReq("bar", "foo"))
	assert.NoError(t, err)
	assert.Greater(t, res.RequeueAfter.Microseconds(), int64(0))

	assert.True(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNRedis_PassCerdentials(t *testing.T) {
	db := newTestVSHNRedis("bar", "foo", "creds")
	cred := newTestVSHNRedisCred("bar", "creds")
	cred.Data = map[string][]byte{
		"REDIS_USER":     []byte("userfoo"),
		"REDIS_PASSWORD": []byte("password"),
		"REDIS_HOST":     []byte("foo.bar"),
		"REDIS_PORT":     []byte("5433"),
		"REDIS_DB":       []byte("pg"),
		"ca.crt":         ca_crt,
		"tls.key":        tls_key,
		"tls.crt":        tls_crt,
	}
	r, manager, client := setupVSHNRedisTest(t,
		db,
		cred,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bar",
				Labels: map[string]string{
					"appuio.io/organization": "bar",
					"appcat.vshn.io/sla":     "besteffort",
				},
			},
		},
	)
	r.RedisDialer = func(service, name, namespace, organization string, opts redis.Options) (*probes.VSHNRedis, error) {

		assert.Equal(t, "VSHNRedis", service)
		assert.Equal(t, "foo", name)
		assert.Equal(t, "bar", namespace)
		assert.Equal(t, "bar", organization)

		certPair, err := tls.X509KeyPair(cred.Data["tls.crt"], cred.Data["tls.key"])
		if err != nil {
			return nil, err
		}
		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{certPair},
			RootCAs:      x509.NewCertPool(),
			// this line must be set to true otherwise probe fails with:
			// "failed to verify certificate: x509: certificate is valid for vshn.appcat.vshn.ch, not redis-headless.instance-name.svc.cluster.local"
			InsecureSkipVerify: true,
		}

		return fakeRedisDialer(service, name, namespace, organization, redis.Options{
			Addr:      string(cred.Data["REDIS_HOST"]) + ":" + string(cred.Data["REDIS_PORT"]),
			Username:  string(cred.Data["REDIS_USERNAME"]),
			Password:  string(cred.Data["REDIS_PASSWORD"]),
			TLSConfig: &tlsConfig,
		})
	}
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}
	pi := probes.ProbeInfo{
		Service:   "VSHNRedis",
		Name:      "foo",
		Namespace: "bar",
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	require.NoError(t, client.Delete(context.TODO(), db))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func fakeRedisDialer(service, name, namespace, organization string, opts redis.Options) (*probes.VSHNRedis, error) {
	p := &probes.VSHNRedis{
		Service:      service,
		Instance:     name,
		Namespace:    namespace,
		Organization: organization,
	}
	return p, nil
}

var _ probeManager = &fakeProbeManager{}

type fakeProbeManager struct {
	probers map[key]bool
}

func newFakeProbeManager() *fakeProbeManager {
	return &fakeProbeManager{
		probers: map[key]bool{},
	}
}

type key string

// StartProbe implements probeManager
func (m *fakeProbeManager) StartProbe(p probes.Prober) {
	m.probers[getFakeKey(p.GetInfo())] = true
}

// StopProbe implements probeManager
func (m *fakeProbeManager) StopProbe(p probes.ProbeInfo) {
	m.probers[getFakeKey(p)] = false
}

func getFakeKey(pi probes.ProbeInfo) key {
	return key(fmt.Sprintf("%s; %s; %s", pi.Service, pi.Namespace, pi.Name))
}

func setupVSHNRedisTest(t *testing.T, objs ...client.Object) (VSHNRedisReconciler, *fakeProbeManager, client.Client) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, vshnv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	manager := newFakeProbeManager()
	r := VSHNRedisReconciler{
		Client:             client,
		Scheme:             scheme,
		ProbeManager:       manager,
		StartupGracePeriod: 5 * time.Minute,
		RedisDialer:        fakeRedisDialer,
	}

	return r, manager, client
}

func newTestVSHNRedis(namespace, name, cred string) *vshnv1.VSHNRedis {
	return &vshnv1.VSHNRedis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: vshnv1.VSHNRedisSpec{
			WriteConnectionSecretToRef: v1.LocalObjectReference{
				Name: cred,
			},
		},
	}
}
func newTestVSHNRedisCred(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"REDIS_USER":     []byte("user"),
			"REDIS_PASSWORD": []byte("password"),
			"REDIS_HOST":     []byte("foo.bar"),
			"REDIS_PORT":     []byte("5432"),
			"REDIS_DB":       []byte("pg"),
			"ca.crt":         ca_crt,
			"tls.key":        tls_key,
			"tls.crt":        tls_crt,
		},
	}
}

func recReq(namespace, name string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}
