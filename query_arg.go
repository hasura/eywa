package eywa

import (
	"fmt"
	"reflect"
	"strings"
)

type queryArgs[M Model, FN FieldName[M], F Field[M]] struct {
	limit      *limit
	offset     *offset
	distinctOn *distinctOn[M, FN]
	where      *where
	orderBy    *orderBy
	set        *set[M, F]
}

func (qa queryArgs[M, FN, F]) MarshalGQL() string {
	var args []string
	args = appendArg(args, qa.limit)
	args = appendArg(args, qa.offset)
	args = appendArg(args, qa.distinctOn)
	args = appendArg(args, qa.where)
	args = appendArg(args, qa.orderBy)
	args = appendArg(args, qa.set)

	return fmt.Sprintf("(%s)", strings.Join(args, ", "))
}

func appendArg(arr []string, arg queryArg) []string {
	if arg == nil || reflect.ValueOf(arg).IsNil() {
		return arr
	}
	s := arg.MarshalGQL()
	if s == "" {
		return arr
	}
	return append(arr, s)
}

type queryArg interface {
	queryArgName() string
	MarshalGQL() string
}

type limit int

func (l limit) queryArgName() string {
	return "limit"
}
func (l limit) MarshalGQL() string {
	return fmt.Sprintf("%s: %d", l.queryArgName(), l)
}

type offset int

func (o offset) queryArgName() string {
	return "offset"
}
func (o offset) MarshalGQL() string {
	return fmt.Sprintf("%s: %d", o.queryArgName(), o)
}

type distinctOn[M Model, FN FieldName[M]] struct {
	field FN
}

func (do distinctOn[M, FN]) queryArgName() string {
	return "distinct_on"
}
func (do distinctOn[M, FN]) MarshalGQL() string {
	return fmt.Sprintf("%s: %s", do.queryArgName(), do.field)
}

type where struct {
	*WhereExpr
}

func (w where) queryArgName() string {
	return "where"
}
func (w where) MarshalGQL() string {
	return fmt.Sprintf("%s: %s", w.queryArgName(), w.WhereExpr.MarshalGQL())
}

type set[M Model, F Field[M]] struct {
	fieldArr[M, F]
}

func (s set[M, F]) queryArgName() string {
	return "_set"
}
func (s set[M, F]) MarshalGQL() string {
	if s.fieldArr == nil || len(s.fieldArr) == 0 {
		return ""
	}
	return fmt.Sprintf("%s: {%s}", s.queryArgName(), s.fieldArr.MarshalGQL())
}

type operator string

const (
	eq  operator = "_eq"
	neq operator = "_neq"
	gt  operator = "_gt"
	gte operator = "_gte"
	lt  operator = "_lt"
	lte operator = "_lte"
)

func compare[M Model, F Field[M]](oprtr operator, field F) *WhereExpr {
	return &WhereExpr{
		cmp: fmt.Sprintf("%s: {%s: %s}", field.GetName(), oprtr, field.GetValue()),
	}
}

func Eq[M Model, F Field[M]](field F) *WhereExpr {
	return compare[M](eq, field)
}

func Neq[M Model, F Field[M]](field F) *WhereExpr {
	return compare[M](neq, field)
}

func Gt[M Model, F Field[M]](field F) *WhereExpr {
	return compare[M](gt, field)
}

func Gte[M Model, F Field[M]](field F) *WhereExpr {
	return compare[M](gte, field)
}

func Lt[M Model, F Field[M]](field F) *WhereExpr {
	return compare[M](lt, field)
}

func Lte[M Model, F Field[M]](field F) *WhereExpr {
	return compare[M](lte, field)
}

func Not(w *WhereExpr) *WhereExpr {
	return &WhereExpr{
		not: w,
	}
}

func And(w ...*WhereExpr) *WhereExpr {
	return &WhereExpr{
		and: whereArr(w),
	}
}

func Or(w ...*WhereExpr) *WhereExpr {
	return &WhereExpr{
		or: whereArr(w),
	}
}

type WhereExpr struct {
	and whereArr
	or  whereArr
	not *WhereExpr
	cmp string
}

type whereArr []*WhereExpr

func (wa whereArr) MarshalGQL() string {
	stringArr := make([]string, 0, len(wa))
	for _, whereExpr := range wa {
		expr := whereExpr.MarshalGQL()
		if expr != "" {
			stringArr = append(stringArr, expr)
		}
	}
	return strings.Join(stringArr, ", ")
}

func (w *WhereExpr) MarshalGQL() string {
	if w == nil {
		return ""
	}
	if (w == &WhereExpr{}) {
		return "{}"
	}
	var stringArr []string

	andExpr := w.and.MarshalGQL()
	if andExpr != "" {
		stringArr = append(stringArr, fmt.Sprintf("_and: [%s]", andExpr))
	}

	orExpr := w.or.MarshalGQL()
	if orExpr != "" {
		stringArr = append(stringArr, fmt.Sprintf("_or: [%s]", orExpr))
	}

	notExpr := w.not.MarshalGQL()
	if notExpr != "" {
		stringArr = append(stringArr, fmt.Sprintf("_not: %s", notExpr))
	}

	if w.cmp != "" {
		stringArr = append(stringArr, w.cmp)
	}
	expr := fmt.Sprintf("{%s}", strings.Join(stringArr, ", "))
	return expr
}

type OrderByExpr struct {
	order string
	field string
}

func (ob OrderByExpr) MarshalGQL() string {
	return fmt.Sprintf("%s: %s", ob.field, ob.order)
}

func Asc[M Model, FN FieldName[M]](field FN) OrderByExpr {
	return OrderByExpr{"asc", string(field)}
}
func AscNullsFirsst[M Model, FN FieldName[M]](field FN) OrderByExpr {
	return OrderByExpr{"asc_nulls_first", string(field)}
}
func AscNullsLast[M Model, FN FieldName[M]](field FN) OrderByExpr {
	return OrderByExpr{"asc_nulls_last", string(field)}
}
func Desc[M Model, FN FieldName[M]](field FN) OrderByExpr {
	return OrderByExpr{"desc", string(field)}
}
func DescNullsFirst[M Model, FN FieldName[M]](field FN) OrderByExpr {
	return OrderByExpr{"desc_nulls_first", string(field)}
}
func DescNullsLast[M Model, FN FieldName[M]](field FN) OrderByExpr {
	return OrderByExpr{"desc_nulls_last", string(field)}
}

type orderBy []OrderByExpr

func (oba orderBy) queryArgName() string {
	return "order_by"
}

func (oba orderBy) MarshalGQL() string {
	if len(oba) == 0 {
		return ""
	}
	stringArr := make([]string, 0, len(oba))
	for _, ob := range oba {
		expr := ob.MarshalGQL()
		if expr != "" {
			stringArr = append(stringArr, expr)
		}
	}
	return fmt.Sprintf("%s: {%s}", oba.queryArgName(), strings.Join(stringArr, ", "))
}
