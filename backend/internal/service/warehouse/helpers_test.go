package warehouse_test

// mainWarehouseID is the fixed id of the migration-seeded default warehouse
// ("MAIN" / "Gudang Utama") — see migrations/sqlite/00001_init.sql. Tests use it
// to exercise default-warehouse preconditions without creating a second row.
const mainWarehouseID = "00000000-0000-0000-0000-0000000000a1"
