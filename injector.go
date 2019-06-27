// Package injector provide a way to wrap arbitary middleware to standard http middleware.
//
// NOTE: that this package heavily use reflection, so the execution time will be *VERY* slow.
// But it is good enough if you compare to the rest of your program.
package injector

import (
	"net/http"
	"reflect"

	"github.com/payfazz/go-middleware/common/kv"
)

type keyType struct{}

var key keyType

func getValuesMap(r *http.Request) (ret map[reflect.Type]reflect.Value) {
	retInterface, ok := kv.Get(r, key)
	if ok {
		ret = retInterface.(map[reflect.Type]reflect.Value)
	} else {
		ret = make(map[reflect.Type]reflect.Value)
		kv.Set(r, key, ret)
	}
	return
}

// StopChain is type that used to stop middleware chain propagation, so next middleware will not be called.
type StopChain func()

// AfterNext is type that used to define action that happen after next middleware called.
type AfterNext func(f func())

var (
	responseType  = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
	requestType   = reflect.TypeOf((**http.Request)(nil)).Elem()
	stopChainType = reflect.TypeOf((*StopChain)(nil)).Elem()
	afterNextType = reflect.TypeOf((*AfterNext)(nil)).Elem()
)

// Wrap is same as Use.
//
// Deprecated: use Use
func Wrap(f interface{}) func(http.HandlerFunc) http.HandlerFunc {
	return Use(f)
}

// Use f as middleware.
//
// f must be a func, it can be any func that receive any parameter and return any value,
// returned values will be remembered (only one item per type) and will be injected to next middleware parameter.
//
// Some special type that will avalable for every middleware parameter:
// `http.ResponseWriter`, `*http.Request`, `injector.StopChain` and `AfterNext`, it cannot be changed.
//
// It require github.com/payfazz/go-middleware/common/kv middleware is already installed in the middleware chain,
// it will panic if kv middleware is not installed in middleware chain.
func Use(f interface{}) func(http.HandlerFunc) http.HandlerFunc {
	fValue, fInType := helper(f)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			valuesMap := getValuesMap(r)

			var skipNext = false
			var after func()

			fIn := make([]reflect.Value, len(fInType))

			for i := 0; i < len(fIn); i++ {
				var t = fInType[i]
				var v reflect.Value

				switch {
				case t == responseType:
					v = reflect.ValueOf(w)

				case t == requestType:
					v = reflect.ValueOf(r)

				case t == stopChainType:
					v = reflect.ValueOf(StopChain(func() {
						skipNext = true
					}))

				case t == afterNextType:
					v = reflect.ValueOf(AfterNext(func(f func()) {
						after = f
					}))

				default:
					if vTmp, ok := valuesMap[t]; ok {
						v = vTmp
					} else {
						v = reflect.Zero(t)
					}
				}
				fIn[i] = v
			}

			fOut := fValue.Call(fIn)
			for _, item := range fOut {
				t := item.Type()
				switch t {
				case responseType, requestType, stopChainType, afterNextType:
				default:
					valuesMap[t] = item
				}
			}

			if !skipNext && next != nil {
				next(w, r)
			}

			if after != nil {
				after()
			}
		}
	}
}

// Handler is same as Use, but used for last middleware,
// next middleware in the chain will not be called.
func Handler(f interface{}) http.HandlerFunc {
	fValue, fInType := helper(f)

	return func(w http.ResponseWriter, r *http.Request) {
		valuesMap := getValuesMap(r)

		fIn := make([]reflect.Value, len(fInType))

		for i := 0; i < len(fIn); i++ {
			var t = fInType[i]
			var v reflect.Value

			switch {
			case t == responseType:
				v = reflect.ValueOf(w)

			case t == requestType:
				v = reflect.ValueOf(r)

			default:
				if vTmp, ok := valuesMap[t]; ok {
					v = vTmp
				} else {
					v = reflect.Zero(t)
				}
			}
			fIn[i] = v
		}

		fOut := fValue.Call(fIn)
		for _, item := range fOut {
			t := item.Type()
			switch t {
			case responseType, requestType, stopChainType, afterNextType:
			default:
				valuesMap[t] = item
			}
		}
	}
}

func helper(f interface{}) (reflect.Value, []reflect.Type) {
	if f == nil {
		panic("injector: f can't be nil")
	}

	fValue := reflect.ValueOf(f)

	fType := fValue.Type()
	if fType.Kind() != reflect.Func {
		panic("injector: f must be a function")
	}

	fInType := make([]reflect.Type, fType.NumIn())
	for i := 0; i < len(fInType); i++ {
		fInType[i] = fType.In(i)
	}
	fInTypeSet := make(map[reflect.Type]struct{})
	for _, item := range fInType {
		if _, ok := fInTypeSet[item]; !ok {
			fInTypeSet[item] = struct{}{}
		} else {
			panic("injector: every parameter of f must have unique type")
		}
	}

	fOutType := make([]reflect.Type, fType.NumOut())
	for i := 0; i < len(fOutType); i++ {
		fOutType[i] = fType.Out(i)
	}
	fOutTypeSet := make(map[reflect.Type]struct{})
	for _, item := range fOutType {
		if _, ok := fOutTypeSet[item]; !ok {
			fOutTypeSet[item] = struct{}{}
		} else {
			panic("injector: every return of f must have unique type")
		}
	}

	return fValue, fInType
}
