-- Sole statement so DROP INDEX CONCURRENTLY stays outside a transaction.
DROP INDEX CONCURRENTLY IF EXISTS idx_flight_performance_carrier_flight_number_date;
