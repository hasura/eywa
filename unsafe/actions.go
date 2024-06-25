package unsafe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/imperfect-fourth/eywa"
)

type ActionQueryBuilder[M eywa.Model] struct {
	ModelName string
	queryArgs map[string]interface{}
}

func Action[M eywa.Model, MP eywa.ModelPtr[M]]() ActionQueryBuilder[M] {
	return ActionQueryBuilder[M]{
		ModelName: (*new(M)).ModelName(),
	}
}

func (aq ActionQueryBuilder[M]) MarshalGQL() string {
	buf := bytes.NewBuffer([]byte{})
	first := true
	for k, v := range aq.queryArgs {
		if !first {
			buf.WriteString(", ")
		} else {
			first = false
		}
		buf.WriteString(k)
		buf.WriteString(": ")
		if val, ok := v.(eywa.GQLMarshaler); ok {
			buf.WriteString(val.MarshalGQL())
		} else {
			val, _ := json.Marshal(v)
			vt := reflect.TypeOf(v)
			if vt.Kind() == reflect.Ptr {
				vt = vt.Elem()
			}
			if vt.Kind() == reflect.Struct || vt.Kind() == reflect.Map {
				val, _ = json.Marshal(string(val))
			}
			buf.WriteString(string(val))
		}
	}

	s := buf.String()
	if s == "" {
		return aq.ModelName
	}
	return fmt.Sprintf("%s(%s)", aq.ModelName, s)
}

func (aq ActionQueryBuilder[M]) QueryArgs(args map[string]interface{}) ActionQueryBuilder[M] {
	aq.queryArgs = args
	return aq
}

func (aq ActionQueryBuilder[M]) Select(field string, fields ...string) ActionQuery[M] {
	return ActionQuery[M]{
		aq:     &aq,
		fields: append(fields, field),
	}
}

type ActionQuery[M eywa.Model] struct {
	aq     *ActionQueryBuilder[M]
	fields []string
}

func (aq ActionQuery[M]) MarshalGQL() string {
	return fmt.Sprintf(
		"%s {\n%s\n}",
		aq.aq.MarshalGQL(),
		strings.Join(aq.fields, "\n"),
	)
}

func (aq ActionQuery[M]) Query() string {
	return fmt.Sprintf(
		"query run_%s {\n%s\n}",
		aq.aq.ModelName,
		aq.MarshalGQL(),
	)
}

func (aq ActionQuery[M]) Variables() map[string]interface{} {
	return nil
}

func (aq ActionQuery[M]) Exec(client *eywa.Client) (*M, error) {
	respBytes, err := client.Do(aq)
	if err != nil {
		return nil, err
	}

	type graphqlResponse struct {
		Data   map[string]*M       `json:"data"`
		Errors []eywa.GraphQLError `json:"errors"`
	}

	respObj := graphqlResponse{}
	err = json.NewDecoder(respBytes).Decode(&respObj)
	if err != nil {
		return nil, err
	}

	if len(respObj.Errors) > 0 {
		gqlErrs := make([]error, 0, len(respObj.Errors))
		for _, e := range respObj.Errors {
			gqlErrs = append(gqlErrs, errors.New(e.Message))
		}
		return nil, errors.Join(gqlErrs...)
	}

	return respObj.Data[aq.aq.ModelName], nil
}
