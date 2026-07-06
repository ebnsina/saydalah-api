-- name: SalesSummary :one
SELECT
    count(*)::bigint                    AS sale_count,
    COALESCE(SUM(total), 0)::numeric    AS revenue,
    COALESCE(SUM(discount), 0)::numeric AS discount_total
FROM sales
WHERE branch_id = $1 AND created_at >= sqlc.arg('from') AND created_at < sqlc.arg('to');

-- name: SalesDaily :many
SELECT
    (date_trunc('day', created_at))::date AS day,
    count(*)::bigint                      AS sale_count,
    COALESCE(SUM(total), 0)::numeric      AS revenue
FROM sales
WHERE branch_id = $1 AND created_at >= sqlc.arg('from') AND created_at < sqlc.arg('to')
GROUP BY day
ORDER BY day;

-- name: SalesByPayment :many
SELECT
    payment_method,
    count(*)::bigint                 AS sale_count,
    COALESCE(SUM(total), 0)::numeric AS revenue
FROM sales
WHERE branch_id = $1
  AND created_at >= sqlc.arg('from') AND created_at < sqlc.arg('to')
  AND voided_at IS NULL
GROUP BY payment_method
ORDER BY revenue DESC;

-- name: InventoryValuation :one
SELECT
    COALESCE(SUM(quantity), 0)::bigint               AS total_units,
    COALESCE(SUM(quantity * cost_price), 0)::numeric AS cost_value,
    COALESCE(SUM(quantity * sale_price), 0)::numeric AS retail_value
FROM stock_batches
WHERE branch_id = $1;

-- name: TopSellingProducts :many
SELECT
    si.product_id,
    p.name                                     AS product_name,
    SUM(si.qty)::bigint                        AS units_sold,
    COALESCE(SUM(si.qty * si.unit_price), 0)::numeric AS revenue
FROM sale_items si
JOIN sales s    ON s.id = si.sale_id
JOIN products p ON p.id = si.product_id
WHERE s.branch_id = $1 AND s.created_at >= sqlc.arg('from') AND s.created_at < sqlc.arg('to')
GROUP BY si.product_id, p.name
ORDER BY units_sold DESC
LIMIT sqlc.arg('limit');
