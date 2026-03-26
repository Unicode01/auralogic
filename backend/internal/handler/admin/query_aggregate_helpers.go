package admin

import "fmt"

func aggregateCountExpr(condition string, alias string) string {
	return fmt.Sprintf("COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) AS %s", condition, alias)
}

func aggregateSumExpr(valueExpr string, condition string, alias string) string {
	return fmt.Sprintf("COALESCE(SUM(CASE WHEN %s THEN %s ELSE 0 END), 0) AS %s", condition, valueExpr, alias)
}
