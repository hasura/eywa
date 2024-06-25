package eywa

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type GraphQLError struct {
	Message    string                 `json:"message"`
	Extensions map[string]interface{} `json:"extensions"`
}

type Model interface {
	ModelName() string
}

type ModelPtr[T Model] interface {
	*T
	Model
}

type ModelFieldName[M Model] string
type FieldName[M Model] interface {
	string | ModelFieldName[M]
}
type FieldNameArr[M Model, FN FieldName[M]] []FN

func (fa FieldNameArr[M, FN]) MarshalGQL() string {
	buf := bytes.NewBufferString("")
	for i, f := range fa {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(string(f))
	}
	return buf.String()
}

type RawField struct {
	Name  string
	Value interface{}
}

func (f RawField) GetName() string {
	return f.Name
}
func (f RawField) GetValue() string {
	if val, ok := f.Value.(GQLMarshaler); ok {
		return val.MarshalGQL()
	}
	val, _ := json.Marshal(f.Value)
	vt := reflect.TypeOf(f.Value)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}
	if vt.Kind() == reflect.Struct || vt.Kind() == reflect.Map {
		val, _ = json.Marshal(string(val))
	}
	return string(val)
}
func (f RawField) GetRawValue() interface{} {
	return f.Value
}

type ModelField[M Model] struct {
	Name  string
	Value interface{}
}

func (f ModelField[M]) GetName() string {
	return f.Name
}
func (f ModelField[M]) GetValue() string {
	if var_, ok := f.Value.(queryVar); ok {
		return fmt.Sprintf("$%s", var_.name)
	}

	if val, ok := f.Value.(GQLMarshaler); ok {
		return val.MarshalGQL()
	}

	val, _ := json.Marshal(f.Value)
	vt := reflect.TypeOf(f.Value)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}
	if vt.Kind() == reflect.Struct || vt.Kind() == reflect.Map {
		val, _ = json.Marshal(string(val))
	}
	return string(val)
}
func (f ModelField[M]) GetRawValue() interface{} {
	return f.Value
}

type Field[M Model] interface {
	RawField | ModelField[M]
	GetName() string
	GetValue() string
	GetRawValue() interface{}
}

type fieldArr[M Model, F Field[M]] []F

func (fs fieldArr[M, MF]) MarshalGQL() string {
	buf := bytes.NewBufferString("")
	for i, f := range fs {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(string(f.GetName()))
		buf.WriteString(": ")
		buf.WriteString(f.GetValue())
	}
	return buf.String()
}

//func RawField[M Model](s string, v interface{}) Field[M] {
//	return Field[M]{s, v}
//}

type Queryable interface {
	Query() string
	Variables() map[string]interface{}
}

type QuerySkeleton[M Model, FN FieldName[M], F Field[M]] struct {
	ModelName string
	queryVars queryVarArr
	// fields    ModelFieldArr[M, FN, F]
	queryArgs[M, FN, F]
}

func (qs QuerySkeleton[M, FN, F]) MarshalGQL() string {
	return fmt.Sprintf("%s%s", qs.ModelName, qs.queryArgs.MarshalGQL())
}

func Get[M Model, MP ModelPtr[M]]() GetQueryBuilder[M, ModelFieldName[M], ModelField[M]] {
	return GetQueryBuilder[M, ModelFieldName[M], ModelField[M]]{
		QuerySkeleton: QuerySkeleton[M, ModelFieldName[M], ModelField[M]]{
			ModelName: (*new(M)).ModelName(),
			//			fields:    append(fields, field),
		},
	}
}

type GetQueryBuilder[M Model, FN FieldName[M], F Field[M]] struct {
	QuerySkeleton[M, FN, F]
}

func (sq GetQueryBuilder[M, FN, F]) DistinctOn(f FN) GetQueryBuilder[M, FN, F] {
	sq.distinctOn = &distinctOn[M, FN]{f}
	return sq
}

func (sq GetQueryBuilder[M, FN, F]) Offset(n int) GetQueryBuilder[M, FN, F] {
	sq.offset = (*offset)(&n)
	return sq
}

func (sq GetQueryBuilder[M, FN, F]) Limit(n int) GetQueryBuilder[M, FN, F] {
	sq.limit = (*limit)(&n)
	return sq
}

func (sq GetQueryBuilder[M, FN, F]) OrderBy(o ...OrderByExpr) GetQueryBuilder[M, FN, F] {
	orderByArr := orderBy(o)
	sq.orderBy = &orderByArr
	return sq
}

func (sq GetQueryBuilder[M, FN, F]) Where(w *WhereExpr) GetQueryBuilder[M, FN, F] {
	sq.where = &where{w}
	return sq
}

func (sq GetQueryBuilder[M, FN, F]) MarshalGQL() string {
	return sq.QuerySkeleton.MarshalGQL()
}

func (sq GetQueryBuilder[M, FN, F]) Select(field FN, fields ...FN) GetQuery[M, FN, F] {
	return GetQuery[M, FN, F]{
		sq:     &sq,
		fields: append(fields, field),
	}
}

type GetQuery[M Model, FN FieldName[M], F Field[M]] struct {
	sq     *GetQueryBuilder[M, FN, F]
	fields []FN
}

func (sq GetQuery[M, FN, F]) MarshalGQL() string {
	return fmt.Sprintf(
		"%s {\n%s\n}",
		sq.sq.MarshalGQL(),
		FieldNameArr[M, FN](sq.fields).MarshalGQL(),
	)
}

func (sq GetQuery[M, FN, F]) Query() string {
	return fmt.Sprintf(
		"query get_%s {\n%s\n}",
		sq.sq.ModelName,
		sq.MarshalGQL(),
	)
}

func (sq GetQuery[M, FN, F]) Variables() map[string]interface{} {
	return nil
}

func (sq GetQuery[M, FN, F]) Exec(client *Client) ([]M, error) {
	respBytes, err := client.Do(sq)
	if err != nil {
		return nil, err
	}

	type graphqlResponse struct {
		Data   map[string][]M `json:"data"`
		Errors []GraphQLError `json:"errors"`
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

	return respObj.Data[sq.sq.ModelName], nil
}
