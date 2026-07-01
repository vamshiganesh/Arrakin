-- name: GetInvestmentByID :one
SELECT *
FROM investments
WHERE id = $1;
