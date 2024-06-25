package eywa

import (
	"encoding/json"
	"fmt"
)

func Update[M Model, MP ModelPtr[M]]() UpdateQueryBuilder[M, ModelFieldName[M], ModelField[M]] {
	return UpdateQueryBuilder[M, ModelFieldName[M], ModelField[M]]{
		QuerySkeleton: QuerySkeleton[M, ModelFieldName[M], ModelField[M]]{
			ModelName: (*new(M)).ModelName(),
			//			fields:    append(fields, field),
		},
	}
}

type UpdateQueryBuilder[M Model, FN FieldName[M], F Field[M]] struct {
	QuerySkeleton[M, FN, F]
}

func (uq UpdateQueryBuilder[M, FN, F]) Set(fields ...F) UpdateQueryBuilder[M, FN, F] {
	uq.set = &set[M, F]{fieldArr[M, F](fields)}
	for _, f := range fields {
		if var_, ok := f.GetRawValue().(queryVar); ok {
			uq.queryVars = append(uq.queryVars, var_)
		}
	}
	return uq
}

func (uq UpdateQueryBuilder[M, FN, F]) Where(w *WhereExpr) UpdateQueryBuilder[M, FN, F] {
	uq.where = &where{w}
	return uq
}

func (uq *UpdateQueryBuilder[M, FN, F]) MarshalGQL() string {
	if uq.where == nil {
		uq.where = &where{Not(&WhereExpr{})}
	}
	return fmt.Sprintf(
		"update_%s",
		uq.QuerySkeleton.MarshalGQL(),
	)
}

func (uq UpdateQueryBuilder[M, FN, F]) Select(field FN, fields ...FN) UpdateQuery[M, FN, F] {
	return UpdateQuery[M, FN, F]{
		uq:     &uq,
		fields: append(fields, field),
	}
}

type UpdateQuery[M Model, FN FieldName[M], F Field[M]] struct {
	uq     *UpdateQueryBuilder[M, FN, F]
	fields []FN
}

func (uq UpdateQuery[M, FN, F]) MarshalGQL() string {
	return fmt.Sprintf(
		"%s {\nreturning {\n%s\n}\n}",
		uq.uq.MarshalGQL(),
		FieldNameArr[M, FN](uq.fields).MarshalGQL(),
	)
}

func (uq UpdateQuery[M, FN, F]) Query() string {
	return fmt.Sprintf(
		"mutation update_%s%s {\n%s\n}",
		uq.uq.ModelName,
		uq.uq.queryVars.MarshalGQL(),
		uq.MarshalGQL(),
	)
}

func (uq UpdateQuery[M, FN, F]) Variables() map[string]interface{} {
	vars := map[string]interface{}{}
	for _, var_ := range uq.uq.queryVars {
		vars[var_.name] = var_.value.Value()
	}
	return vars
}

func (uq UpdateQuery[M, FN, F]) Exec(client *Client) ([]M, error) {
	respBytes, err := client.Do(uq)
	if err != nil {
		return nil, err
	}

	type mutationReturning struct {
		Returning []M `json:"returning"`
	}
	type graphqlResponse struct {
		Data   map[string]mutationReturning `json:"data"`
		Errors []GraphQLError               `json:"errors"`
	}

	respObj := graphqlResponse{}

	err = json.NewDecoder(respBytes).Decode(&respObj)
	if err != nil {
		return nil, err
	}
	return respObj.Data[fmt.Sprintf("update_%s", uq.uq.ModelName)].Returning, nil
}
