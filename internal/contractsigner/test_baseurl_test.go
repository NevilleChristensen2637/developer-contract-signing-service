package contractsigner

import (
	"reflect"
	"unsafe"

	"github.com/example/developer-contract-signing-service/internal/infrai"
)

func clientReflectSetBaseURL(client *infrai.Client, baseURL string) {
	v := reflect.ValueOf(client).Elem().FieldByName("baseURL")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().SetString(baseURL)
}
