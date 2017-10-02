package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apoydence/pubsub/pubsub-gen/internal/inspector"
)

type CodeWriter struct{}

func (w CodeWriter) Package(name string) string {
	return fmt.Sprintf("package %s\n\n", name)
}

func (w CodeWriter) Imports(names []string) string {
	result := "import (\n"
	for _, n := range names {
		if n == "" {
			continue
		}
		result += fmt.Sprintf("  \"%s\"\n", n)
	}
	return fmt.Sprintf("%s)\n", result)
}

func (w CodeWriter) Traverse(travName, firstField string) string {
	return fmt.Sprintf(`
func %sTraverse(data interface{}) pubsub.Paths {
	return _%s(data)
}
`, travName, firstField)
}

func (w CodeWriter) Done(travName string) string {
	return `
	func done(data interface{}) pubsub.Paths {
	return pubsub.Paths( func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
	  return 0, nil, false
	})
}
`
}

func (w CodeWriter) Hashers(travName string) string {
	return `
func hashBool(data bool) uint64 {
	if data {
		return 1
	}
	return 0
}

var tableECMA = crc64.MakeTable(crc64.ECMA)
`

}

func (w CodeWriter) FieldStartStruct(travName, prefix, fieldName, parentFieldName, castTypeName string, isPtr bool, enumValue int) string {
	var nilCheck string
	if isPtr {
		nilCheck = fmt.Sprintf(`
  if %s == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(done), true
			default:
				return 0, nil, false
			}
		})
  }
		`, castTypeName)
	}

	return fmt.Sprintf(`
func %s(data interface{}) pubsub.Paths {
	%s
  return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
			switch idx {
			case 0:
				return %d, pubsub.TreeTraverser(%s_%s), true
			default:
				return 0, nil, false
			}
		})
}
`, prefix, nilCheck, enumValue, prefix, fieldName)
}

func (w CodeWriter) FieldSelector(travName, prefix, fieldName, parentFieldName, castTypeName string, isPtr bool, enumValue int) string {
	var nilCheck string
	if isPtr {
		nilCheck = fmt.Sprintf(`
  if %s.%s == nil {
		return 0, pubsub.TreeTraverser(done), true
  }
		`, castTypeName, parentFieldName)
	}

	return fmt.Sprintf(`
	%s
	return %d, pubsub.TreeTraverser(%s_%s), true
`, nilCheck, enumValue, prefix, fieldName)
}

func (w CodeWriter) SelectorFunc(travName, selectorName string, fields []string) string {
	var body string
	for i, f := range fields {
		body += fmt.Sprintf(`
	case %d:
	%s
		`, i, f)
	}

	return fmt.Sprintf(`
	func __%s (idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
		switch idx{
	%s
default:
	return 0, nil, false
}
	}
	`, selectorName, body)
}

func (w CodeWriter) FieldStructFunc(travName, prefix, fieldName, nextFieldName, castTypeName, hashType string, isPtr bool, slice inspector.Slice) string {
	var nilCheck string
	if isPtr || slice.IsSlice {
		nilCheck = fmt.Sprintf(`
  if %s.%s == nil {
    return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(%s_%s), true
			default:
				return 0, nil, false
			}
		})
  }
		`, castTypeName, fieldName, prefix, nextFieldName)
	}

	var star string
	if isPtr {
		star = "*"
	}

	dataValue := fmt.Sprintf("%s%s.%s", star, castTypeName, fieldName)
	hashCalc, hashValue := hashSplitFn(hashType, dataValue, slice)

	return fmt.Sprintf(`
func %s_%s(data interface{}) pubsub.Paths {
	%s
  return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(%s_%s), true
			case 1:
				%s
				return %s, pubsub.TreeTraverser(%s_%s), true
			default:
				return 0, nil, false
			}
		})
}
`, prefix, fieldName, nilCheck, prefix, nextFieldName, hashCalc, hashValue, prefix, nextFieldName)
}

func (w CodeWriter) FieldStructFuncLast(travName, prefix, fieldName, castTypeName, hashType string, isPtr bool, slice inspector.Slice) string {
	var nilCheck string
	if isPtr || slice.IsSlice {
		nilCheck = fmt.Sprintf(`
  if %s.%s == nil {
    return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(done), true
			default:
				return 0, nil, false
			}
		})
  }
		`, castTypeName, fieldName)
	}

	var star string
	if isPtr {
		star = "*"
	}

	dataValue := fmt.Sprintf("%s%s.%s", star, castTypeName, fieldName)
	hashCalc, hashValue := hashSplitFn(hashType, dataValue, slice)

	return fmt.Sprintf(`
func %s_%s(data interface{}) pubsub.Paths {
	%s
  return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(done), true
			case 1:
				%s
				return %s, pubsub.TreeTraverser(done), true
			default:
				return 0, nil, false
			}
		})
}
`, prefix, fieldName, nilCheck, hashCalc, hashValue)
}

func (w CodeWriter) FieldPeersFunc(travName, prefix, castTypeName, fieldName, hashType string, names []string, isPtr bool, slice inspector.Slice) string {
	travFunc := fmt.Sprintf(`
    pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
			return __%s
 		})`, strings.Join(names, "_"))

	var nilCheck string
	if isPtr || slice.IsSlice {
		nilCheck = fmt.Sprintf(`
  if %s.%s == nil {
    return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
			switch idx {
			case 0:
				return 0, %s, true
			default:
				return 0, nil, false
			}
		})
  }
		`, castTypeName, fieldName, travFunc)
	}

	var star string
	if isPtr {
		star = "*"
	}

	dataValue := fmt.Sprintf("%s%s.%s", star, castTypeName, fieldName)
	hashCalc, hashValue := hashSplitFn(hashType, dataValue, slice)

	return fmt.Sprintf(`
func %s_%s(data interface{}) pubsub.Paths {
	%s
  return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool){
		switch idx{
		case 0:
				return 0, %s, true
		case 1:
				%s
				return %s, %s, true
	  default:
			return 0, nil, false
		}
	})
}
`, prefix, fieldName, nilCheck, travFunc, hashCalc, hashValue, travFunc)
}

func (w CodeWriter) InterfaceSelector(prefix, castTypeName, fieldName, structPkgPrefix string, implementers map[string]string, startIdx int) string {
	idxs := orderImpls(implementers)
	body := fmt.Sprintf("switch %s.%s.(type) {", castTypeName, fieldName)
	for i, f := range implementers {
		body += fmt.Sprintf(`
case %s%s:
	return %d, %s_%s_%s_%s, true
`, structPkgPrefix, i, idxs[i]+startIdx, prefix, fieldName, i, f)
	}
	body += `
default:
	return 0, pubsub.TreeTraverser(done), true
}`

	return body
}

func orderImpls(impls map[string]string) map[string]int {
	m := make(map[string]int)

	var names []string
	for k := range impls {
		names = append(names, k)
	}

	sort.Strings(names)

	for i, s := range names {
		m[s] = i + 1
	}

	return m
}
