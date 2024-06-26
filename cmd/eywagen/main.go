package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/types"
	"os"
	"regexp"
	re "regexp"
	"strings"

	"golang.org/x/tools/go/packages"
)

var (
	typeNames  = flag.String("types", "", "comma-separated list of type names; must be set")
	outputFile = flag.String("output-file", "eywa_generated.go", "output file path for generated file.")
)

func usage() {
	fmt.Fprint(os.Stderr, "Usage:")
	fmt.Fprint(os.Stderr, "\teywagen -types <comma separated list of type names>")
}

var tagPattern = re.MustCompile(`json:"([^"]+)"`)

const (
	genHeader           = "// generated by eywa. DO NOT EDIT. Any changes will be overwritten.\npackage "
	modelFieldNameConst = "const %s eywa.ModelFieldName[%s] = \"%s\"\n"
	modelFieldFunc      = `
func %sField(val %s) eywa.ModelField[%s] {
	return eywa.ModelField[%s]{
		Name: "%s",
		Value: val,
	}
}
`
	modelScalarVarFunc = `
func %sVar(val %s) eywa.ModelField[%s] {
	return eywa.ModelField[%s]{
		Name: "%s",
		Value: eywa.QueryVar("%s", %s[%s](val)),
	}
}
`
	modelVarFunc = `
func %sVar[T interface{%s;eywa.TypedValue}](val %s) eywa.ModelField[%s] {
	return eywa.ModelField[%s]{
		Name: "%s",
		Value: eywa.QueryVar("%s", T{val}),
	}
}
`

	modelRelationshipNameFunc = `
func %s(subField eywa.ModelFieldName[%s], subFields ...eywa.ModelFieldName[%s]) string {
	buf := bytes.NewBuffer([]byte("%s {"))
	buf.WriteString(string(subField))
	for _, f := range subFields {
		buf.WriteString("\n")
		buf.WriteString(string(f))
	}
	buf.WriteString("}")
	return buf.String()
}
`
)

func main() {
	flag.Usage = usage
	flag.Parse()
	if *typeNames == "" {
		flag.Usage()
		os.Exit(2)
	}
	types := strings.Split(*typeNames, ",")

	pkg, err := loadPackage()
	if err != nil {
		panic(err)
	}

	header := bytes.NewBufferString(genHeader)
	header.WriteString(pkg.Name())
	header.WriteString("\n")

	contents := &fileContent{
		header:     header,
		importsMap: map[string]bool{"github.com/imperfect-fourth/eywa": true},
		imports:    bytes.NewBuffer([]byte{}),
		content:    bytes.NewBufferString(""),
	}
	for _, t := range types {
		parseType(t, pkg, contents)
	}
	if len(contents.importsMap) > 0 {
		contents.imports.WriteString("\nimport (\n")
		for pkgImport, ok := range contents.importsMap {
			if ok {
				contents.imports.WriteString(fmt.Sprintf("\t\"%s\"\n", pkgImport))
			}
		}
		contents.imports.WriteString(")\n\n")
	}
	if err := writeToFile(*outputFile, contents); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}

type fileContent struct {
	header     *bytes.Buffer
	importsMap map[string]bool
	imports    *bytes.Buffer
	content    *bytes.Buffer
}

var parsed = make(map[string]bool)

func parseType(typeName string, pkg *types.Package, contents *fileContent) {
	if parsed[typeName] {
		return
	}
	parsed[typeName] = true

	typeObj := pkg.Scope().Lookup(typeName)
	if typeObj == nil {
		fmt.Printf("type %s not found in package, skipping...", typeName)
		return
	}
	typeStruct, ok := typeObj.Type().Underlying().(*types.Struct)
	if !ok {
		fmt.Printf("type %s is not a struct, skipping...", typeName)
		return
	}
	if types.NewMethodSet(types.NewPointer(typeObj.Type())).Lookup(pkg, "ModelName") == nil {
		fmt.Printf("struct type %s does not implement eywa.Model interface, skipping...", typeName)
		return
	}

	contents.content.WriteString("\n")
	recurseParse := make([]string, 0, typeStruct.NumFields())
	for i := 0; i < typeStruct.NumFields(); i++ {
		tag := tagPattern.FindStringSubmatch(typeStruct.Tag(i))
		if tag == nil {
			continue
		}
		tagValue := strings.Split(tag[1], ",")
		if len(tagValue) == 0 {
			continue
		}
		fieldName := tagValue[0]
		field := typeStruct.Field(i)
		fieldType := field.Type()
		typeSourcePkgName, fieldTypeNameFull := parseFieldTypeName(field.Type().String(), pkg.Path())
		if typeSourcePkgName != "" {
			contents.importsMap[typeSourcePkgName] = true
		}
		fieldTypeName := fieldTypeNameFull
		if fieldTypeNameFull[0] == '*' {
			fieldTypeName = fieldTypeNameFull[1:]
		}
		fieldScalarGqlType := gqlType(fieldType.Underlying().String())

		// *struct -> struct, *[] -> [], *int -> int, etc
		if ptr, ok := fieldType.(*types.Pointer); ok {
			fieldType = ptr.Elem()
		}
		// []*x -> *x, []x -> x
		if slice, ok := fieldType.(*types.Slice); ok {
			fieldType = slice.Elem()
		} else if array, ok := fieldType.(*types.Array); ok {
			fieldType = array.Elem()
		}
		// struct -> *struct
		var fieldGqlType string
		if _, ok := fieldType.Underlying().(*types.Struct); ok {
			fieldType = types.NewPointer(fieldType)
			fieldGqlType = "eywa.JSONValue | eywa.JSONBValue"
		} else if _, ok := fieldType.Underlying().(*types.Map); ok {
			fieldGqlType = "eywa.JSONValue | eywa.JSONBValue"
		}

		switch fieldType := fieldType.(type) {
		case *types.Pointer:
			fieldMethodSet := types.NewMethodSet(fieldType)
			if m := fieldMethodSet.Lookup(pkg, "ModelName"); m != nil && m.Type().String() == "func() string" {
				contents.importsMap["bytes"] = true
				contents.content.WriteString(fmt.Sprintf(
					modelRelationshipNameFunc,
					fmt.Sprintf("%s_%s", typeName, field.Name()),
					fieldTypeName,
					fieldTypeName,
					fieldName,
				))
				recurseParse = append(recurseParse, fieldTypeName)
			} else {
				contents.content.WriteString(fmt.Sprintf(
					modelFieldNameConst,
					fmt.Sprintf("%s_%s", typeName, field.Name()),
					typeName,
					fieldName,
				))
				contents.content.WriteString(fmt.Sprintf(
					modelFieldFunc,
					fmt.Sprintf("%s_%s", typeName, field.Name()),
					fieldTypeNameFull,
					typeName,
					typeName,
					fieldName,
				))
				if fieldScalarGqlType != "" {
					contents.content.WriteString(fmt.Sprintf(
						modelScalarVarFunc,
						fmt.Sprintf("%s_%s", typeName, field.Name()),
						fieldTypeNameFull,
						typeName,
						typeName,
						fieldName,
						fmt.Sprintf("%s_%s", typeName, field.Name()),
						fmt.Sprintf("eywa.%s", fieldScalarGqlType),
						fieldTypeNameFull,
					))
				} else if fieldGqlType != "" {
					contents.content.WriteString(fmt.Sprintf(
						modelVarFunc,
						fmt.Sprintf("%s_%s", typeName, field.Name()),
						fieldGqlType,
						fieldTypeNameFull,
						typeName,
						typeName,
						fieldName,
						fmt.Sprintf("%s_%s", typeName, field.Name()),
					))
				}
			}
		default:
			contents.content.WriteString(fmt.Sprintf(
				modelFieldNameConst,
				fmt.Sprintf("%s_%s", typeName, field.Name()),
				typeName,
				fieldName,
			))
			contents.content.WriteString(fmt.Sprintf(
				modelFieldFunc,
				fmt.Sprintf("%s_%s", typeName, field.Name()),
				fieldTypeNameFull,
				typeName,
				typeName,
				fieldName,
			))
			if fieldScalarGqlType != "" {
				contents.content.WriteString(fmt.Sprintf(
					modelScalarVarFunc,
					fmt.Sprintf("%s_%s", typeName, field.Name()),
					fieldTypeNameFull,
					typeName,
					typeName,
					fieldName,
					fmt.Sprintf("%s_%s", typeName, field.Name()),
					fmt.Sprintf("eywa.%sVar", fieldScalarGqlType),
					fieldTypeNameFull,
				))
			} else if fieldGqlType != "" {
				contents.content.WriteString(fmt.Sprintf(
					modelVarFunc,
					fmt.Sprintf("%s_%s", typeName, field.Name()),
					fieldGqlType,
					fieldTypeNameFull,
					typeName,
					typeName,
					fieldName,
					fmt.Sprintf("%s_%s", typeName, field.Name()),
				))
			}
		}
	}
	for _, t := range recurseParse {
		parseType(t, pkg, contents)
	}

}

func writeToFile(filename string, contents *fileContent) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := contents.header.WriteTo(f); err != nil {
		return err
	}
	if _, err := contents.imports.WriteTo(f); err != nil {
		return err
	}
	if _, err := contents.content.WriteTo(f); err != nil {
		return err
	}
	return nil
}

func loadPackage() (*types.Package, error) {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo, Tests: true}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("couldn't load package: %v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("package contains errors")
	}
	return pkgs[0].Types, nil
}

func parseFieldTypeName(name, rootPkgPath string) (sourcePkgPath, typeName string) {
	re, _ := regexp.Compile(`^(\*)?(.*/(.*))\.(.*)$`)
	matches := re.FindStringSubmatch(name)
	if len(matches) == 0 {
		return "", name
	}
	if rootPkgPath == matches[2] {
		return "", fmt.Sprintf("%s%s", matches[1], matches[4])
	}
	return matches[2], fmt.Sprintf("%s%s.%s", matches[1], matches[3], matches[4])
}

var gqlTypes = map[string]string{
	"bool":    "Boolean",
	"*bool":   "NullableBoolean",
	"int":     "Int",
	"*int":    "NullableInt",
	"float":   "Float",
	"*float":  "NullableFloat",
	"string":  "String",
	"*string": "NullableString",
}

func gqlType(fieldType string) string {
	for k, v := range gqlTypes {
		if strings.HasPrefix(fieldType, k) {
			return v
		}
	}
	return ""
}
