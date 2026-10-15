-- Sole statement so DROP INDEX CONCURRENTLY stays outside a transaction.
DROP INDEX CONCURRENTLY IF EXISTS idx_flight_performance_origin_dest_carrier_dow_date;
