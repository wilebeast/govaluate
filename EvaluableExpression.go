package govaluate

import (
	"errors"
	"fmt"

	"github.com/wilebeast/govaluate/ellen"
)

const isoDateFormat string = "2006-01-02T15:04:05.999999999Z0700"
const shortCircuitHolder int = -1

var DUMMY_PARAMETERS = MapParameters(map[string]interface{}{})

/*
EvaluableExpression represents a set of ExpressionTokens which, taken together,
are an expression that can be evaluated down into a single value.
*/
type EvaluableExpression struct {

	/*
		Represents the query format used to output dates. Typically only used when creating SQL or Mongo queries from an expression.
		Defaults to the complete ISO8601 format, including nanoseconds.
	*/
	QueryDateFormat string

	/*
		Whether or not to safely check types when evaluating.
		If true, this library will return error messages when invalid types are used.
		If false, the library will panic when operators encounter types they can't use.

		This is exclusively for users who need to squeeze every ounce of speed out of the library as they can,
		and you should only set this to false if you know exactly what you're doing.
	*/
	ChecksTypes bool

	tokens           []ExpressionToken
	evaluationStages *evaluationStage
	inputExpression  string
}

/*
Parses a new EvaluableExpression from the given [expression] string.
Returns an error if the given expression has invalid syntax.
*/
func NewEvaluableExpression(expression string) (X1 *EvaluableExpression, X2 error) {
	defer func() {
		ellen.Printf("NewEvaluableExpression", map[string]interface{}{"expression": expression}, map[string]interface{}{"X1": X1, "X2": X2})

		/*
			Similar to [NewEvaluableExpression], except that instead of a string, an already-tokenized expression is given.
			This is useful in cases where you may be generating an expression automatically, or using some other parser (e.g., to parse from a query language)
		*/
	}()

	functions := make(map[string]ExpressionFunction)
	return NewEvaluableExpressionWithFunctions(expression, functions)
}

func NewEvaluableExpressionFromTokens(tokens []ExpressionToken) (X1 *EvaluableExpression, X2 error) {
	defer func() {
		ellen.Printf("NewEvaluableExpressionFromTokens", map[string]interface{}{"tokens": tokens}, map[string]interface{}{"X1": X1, "X2": X2})
	}()

	var ret *EvaluableExpression
	var err error

	ret = new(EvaluableExpression)
	ret.QueryDateFormat = isoDateFormat

	err = checkBalance(tokens)
	if err != nil {
		return nil, err
	}

	err = checkExpressionSyntax(tokens)
	if err != nil {
		return nil, err
	}

	ret.tokens, err = optimizeTokens(tokens)
	if err != nil {
		return nil, err
	}

	ret.evaluationStages, err = planStages(ret.tokens)
	if err != nil {
		return nil, err
	}

	ret.ChecksTypes = true
	return ret, nil
}

/*
Similar to [NewEvaluableExpression], except enables the use of user-defined functions.
Functions passed into this will be available to the expression.
*/
func NewEvaluableExpressionWithFunctions(expression string, functions map[string]ExpressionFunction) (X1 *EvaluableExpression, X2 error) {
	defer func() {
		ellen.Printf("NewEvaluableExpressionWithFunctions", map[string]interface{}{"expression": expression, "functions": functions}, map[string]interface{}{"X1": X1, "X2": X2})
	}()

	var ret *EvaluableExpression
	var err error

	ret = new(EvaluableExpression)
	ret.QueryDateFormat = isoDateFormat
	ret.inputExpression = expression

	ret.tokens, err = parseTokens(expression, functions)
	if err != nil {
		return nil, err
	}

	err = checkBalance(ret.tokens)
	if err != nil {
		return nil, err
	}

	err = checkExpressionSyntax(ret.tokens)
	if err != nil {
		return nil, err
	}

	ret.tokens, err = optimizeTokens(ret.tokens)
	if err != nil {
		return nil, err
	}

	ret.evaluationStages, err = planStages(ret.tokens)
	if err != nil {
		return nil, err
	}

	ret.ChecksTypes = true
	return ret, nil
}

/*
Same as `Eval`, but automatically wraps a map of parameters into a `govalute.Parameters` structure.
*/
func (this EvaluableExpression) Evaluate(parameters map[string]interface{}) (X1 interface{}, X2 error) {
	defer func() {
		ellen.Printf("Evaluate", map[string]interface{}{"parameters": parameters}, map[string]interface{}{"X1": X1, "X2": X2})

		/*
			Runs the entire expression using the given [parameters].
			e.g., If the expression contains a reference to the variable "foo", it will be taken from `parameters.Get("foo")`.

			This function returns errors if the combination of expression and parameters cannot be run,
			such as if a variable in the expression is not present in [parameters].

			In all non-error circumstances, this returns the single value result of the expression and parameters given.
			e.g., if the expression is "1 + 1", this will return 2.0.
			e.g., if the expression is "foo + 1" and parameters contains "foo" = 2, this will return 3.0
		*/
	}()

	if parameters == nil {
		return this.Eval(nil)
	}

	return this.Eval(MapParameters(parameters))
}

func (this EvaluableExpression) Eval(parameters Parameters) (X1 interface{}, X2 error) {
	defer func() {
		ellen.Printf("Eval", map[string]interface{}{"parameters": parameters}, map[string]interface{}{"X1": X1, "X2": X2})
	}()

	if this.evaluationStages == nil {
		return nil, nil
	}

	if parameters != nil {
		parameters = &sanitizedParameters{parameters}
	} else {
		parameters = DUMMY_PARAMETERS
	}

	return this.evaluateStage(this.evaluationStages, parameters)
}

func (this EvaluableExpression) evaluateStage(stage *evaluationStage, parameters Parameters) (X1 interface{}, X2 error) {
	defer func() {
		ellen.Printf("evaluateStage", map[string]interface{}{"stage": stage, "parameters": parameters}, map[string]interface{}{"X1": X1, "X2": X2})
	}()

	var left, right interface{}
	var err error

	if stage.leftStage != nil {
		left, err = this.evaluateStage(stage.leftStage, parameters)
		if err != nil {
			return nil, err
		}
	}

	if stage.isShortCircuitable() {
		switch stage.symbol {
		case AND:
			if left == false {
				return false, nil
			}
		case OR:
			if left == true {
				return true, nil
			}
		case COALESCE:
			if left != nil {
				return left, nil
			}

		case TERNARY_TRUE:
			if left == false {
				right = shortCircuitHolder
			}
		case TERNARY_FALSE:
			if left != nil {
				right = shortCircuitHolder
			}
		}
	}

	if right != shortCircuitHolder && stage.rightStage != nil {
		right, err = this.evaluateStage(stage.rightStage, parameters)
		if err != nil {
			return nil, err
		}
	}

	if this.ChecksTypes {
		if stage.typeCheck == nil {

			err = typeCheck(stage.leftTypeCheck, left, stage.symbol, stage.typeErrorFormat)
			if err != nil {
				return nil, err
			}

			err = typeCheck(stage.rightTypeCheck, right, stage.symbol, stage.typeErrorFormat)
			if err != nil {
				return nil, err
			}
		} else {
			// special case where the type check needs to know both sides to determine if the operator can handle it
			if !stage.typeCheck(left, right) {
				errorMsg := fmt.Sprintf(stage.typeErrorFormat, left, stage.symbol.String())
				return nil, errors.New(errorMsg)
			}
		}
	}

	return stage.operator(left, right, parameters)
}

func typeCheck(check stageTypeCheck, value interface{}, symbol OperatorSymbol, format string) (X1 error) {
	defer func() {
		ellen.Printf("typeCheck", map[string]interface{}{"check": check, "value": value, "symbol": symbol, "format": format}, map[string]interface{}{"X1": X1})
	}()

	if check == nil {
		return nil
	}

	if check(value) {
		return nil
	}

	errorMsg := fmt.Sprintf(format, value, symbol.String())
	return errors.New(errorMsg)
}

/*
Returns an array representing the ExpressionTokens that make up this expression.
*/
func (this EvaluableExpression) Tokens() (X1 []ExpressionToken) {
	defer func() {
		ellen.Printf("Tokens", map[string]interface{}{}, map[string]interface{}{"X1": X1})

		/*
			Returns the original expression used to create this EvaluableExpression.
		*/
	}()

	return this.tokens
}

func (this EvaluableExpression) String() (X1 string) {
	defer func() {
		ellen.Printf("String", map[string]interface{}{}, map[string]interface{}{"X1": X1})

		/*
			Returns an array representing the variables contained in this EvaluableExpression.
		*/
	}()

	return this.inputExpression
}

func (this EvaluableExpression) Vars() (X1 []string) {
	defer func() {
		ellen.Printf("Vars", map[string]interface{}{}, map[string]interface{}{"X1": X1})
	}()
	var varlist []string
	for _, val := range this.Tokens() {
		if val.Kind == VARIABLE {
			varlist = append(varlist, val.Value.(string))
		}
	}
	return varlist
}
