package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/google/wire"
	"gorm.io/gorm"

	"github.com/justmart/backend/gen/analytics_iface/v1/analyticsifacev1connect"
	"github.com/justmart/backend/gen/backup_iface/v1/backupifacev1connect"
	"github.com/justmart/backend/gen/branch_iface/v1/branchifacev1connect"
	"github.com/justmart/backend/gen/connector_iface/v1/connectorifacev1connect"
	"github.com/justmart/backend/gen/customer_iface/v1/customerifacev1connect"
	"github.com/justmart/backend/gen/health_iface/v1/healthifacev1connect"
	"github.com/justmart/backend/gen/inventory_iface/v1/inventoryifacev1connect"
	"github.com/justmart/backend/gen/pos_iface/v1/posifacev1connect"
	"github.com/justmart/backend/gen/prescription_iface/v1/prescriptionifacev1connect"
	"github.com/justmart/backend/gen/purchasing_iface/v1/purchasingifacev1connect"
	"github.com/justmart/backend/gen/settings_iface/v1/settingsifacev1connect"
	"github.com/justmart/backend/gen/stocktake_iface/v1/stocktakeifacev1connect"
	"github.com/justmart/backend/gen/unit_iface/v1/unitifacev1connect"
	"github.com/justmart/backend/gen/user_iface/v1/userifacev1connect"
	"github.com/justmart/backend/gen/warehouse_iface/v1/warehouseifacev1connect"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/db"
	"github.com/justmart/backend/internal/dbmigrate"
	"github.com/justmart/backend/internal/service/analytics"
	authsvc "github.com/justmart/backend/internal/service/auth"
	"github.com/justmart/backend/internal/service/backup"
	"github.com/justmart/backend/internal/service/batch"
	"github.com/justmart/backend/internal/service/branch"
	"github.com/justmart/backend/internal/service/connector"
	"github.com/justmart/backend/internal/service/customer"
	"github.com/justmart/backend/internal/service/health"
	"github.com/justmart/backend/internal/service/prescription"
	"github.com/justmart/backend/internal/service/priceagreement"
	"github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/productdiscount"
	"github.com/justmart/backend/internal/service/productpricetier"
	"github.com/justmart/backend/internal/service/purchasing"
	"github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/stock"
	"github.com/justmart/backend/internal/service/stocktake"
	"github.com/justmart/backend/internal/service/supplier"
	"github.com/justmart/backend/internal/service/transfer"
	"github.com/justmart/backend/internal/service/unit"
	"github.com/justmart/backend/internal/service/user"
	"github.com/justmart/backend/internal/service/warehouse"
	"github.com/justmart/backend/internal/web"
	cloudflaretunnel "github.com/justmart/backend/pkgs/cloudflare_tunnel"
)

// configPath is the --config flag value, as its own type so Wire can tell it
// apart from every other string in the graph. Empty => $JUSTMART_CONFIG, else
// ./config.yaml (see config.Load).
type configPath string

// buildVersion is main.version (link-time stamped), as its own type for Wire.
type buildVersion string

// Handlers aggregates every ConnectRPC service implementation. Wire fills it
// with wire.Struct(new(Handlers), "*"), so adding a service is: add a field
// here, add its constructor to serviceSet, register it in provideRootHandler.
type Handlers struct {
	Analytics         *analytics.AnalyticsService
	Auth              *authsvc.AuthService
	Backups           *backup.Backups
	Batches           *batch.BatchService
	Branches          *branch.BranchService
	Connectors        *connector.ConnectorService
	Customers         *customer.CustomerService
	Health            *health.HealthService
	PriceAgreements   *priceagreement.PriceAgreementService
	Prescriptions     *prescription.PrescriptionService
	ProductDiscounts  *productdiscount.ProductDiscountService
	ProductPriceTiers *productpricetier.ProductPriceTierService
	Products          *product.ProductService
	PurchaseOrders    *purchasing.PurchaseOrders
	PurchasePayments  *purchasing.PurchasePayments
	PurchaseReceipts  *purchasing.PurchaseReceipts
	PurchaseReturns   *purchasing.PurchaseReturns
	Sales             *sale.SaleService
	Settings          *settings.SettingsService
	Stock             *stock.StockService
	Stocktakes        *stocktake.StocktakeService
	Suppliers         *supplier.SupplierService
	Transfers         *transfer.TransferService
	Units             *unit.UnitService
	Users             *user.UserService
	Warehouses        *warehouse.WarehouseService
}

// appSet is the whole object graph: config + DB + auth primitives + every
// service + the HTTP stack, assembled into an *App.
var appSet = wire.NewSet(
	infraSet,
	authSet,
	serviceSet,
	httpSet,
	wire.Struct(new(App), "*"),
)

// infraSet: config file -> parsed config -> migrated DB pool, plus the optional
// Cloudflare tunnel.
var infraSet = wire.NewSet(
	provideConfig,
	provideDB,
	provideTunnelRunner,
)

// authSet: the JWT/refresh issuers, the declared-in-proto policy table, the
// login rate limiter, and the interceptor chain every handler is mounted with.
var authSet = wire.NewSet(
	provideIssuer,
	provideRefreshIssuer,
	provideAuthPolicy,
	provideLoginLimiter,
	provideInterceptors,
)

// serviceSet: one entry per ConnectRPC service implementation. Constructors
// that only need *gorm.DB are referenced directly; the three that need
// post-construction wiring (connector mode, printer, autoupdater) have a
// provide* wrapper below.
var serviceSet = wire.NewSet(
	analytics.NewAnalyticsService,
	authsvc.NewAuthService,
	backup.NewBackupService,
	batch.NewBatchService,
	branch.NewBranchService,
	customer.NewCustomerService,
	health.NewHealthService,
	prescription.NewPrescriptionService,
	priceagreement.NewPriceAgreementService,
	product.NewProductService,
	productdiscount.NewProductDiscountService,
	productpricetier.NewProductPriceTierService,
	purchasing.NewPurchaseOrderService,
	purchasing.NewPurchasePaymentService,
	purchasing.NewPurchaseReceiptService,
	purchasing.NewPurchaseReturnService,
	stock.NewStockService,
	stocktake.NewStocktakeService,
	supplier.NewSupplierService,
	transfer.NewTransferService,
	unit.NewUnitService,
	user.NewUserService,
	warehouse.NewWarehouseService,
	provideConnectorService,
	provideSaleService,
	provideSettingsService,
	wire.Struct(new(Handlers), "*"),
)

// httpSet: the root mux (Connect API under /api + /healthz + embedded SPA) and
// the http.Server it is served on.
var httpSet = wire.NewSet(
	provideRootHandler,
	provideHTTPServer,
)

func provideConfig(path configPath) (*config.Config, error) {
	cfg, err := config.Load(string(path))
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// provideDB opens the connection pool and hands back a DB whose schema is ready
// to use. Auto-migrate on boot is OFF by default — opt in with
// `auto_migrate: true` (the turnkey Docker / Windows configs do, so a freshly
// deployed binary still brings its own schema up to date). Otherwise migrate
// explicitly via `cmd/server migrate up`. The cleanup closes the pool on
// shutdown.
func provideDB(cfg *config.Config) (*gorm.DB, func(), error) {
	gormDB, err := db.Open(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("get sql.DB: %w", err)
	}
	cleanup := func() { _ = sqlDB.Close() }

	if cfg.Database.ShouldAutoMigrate() {
		if err := dbmigrate.Run(sqlDB, cfg.Database.DriverName()); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("auto-migrate: %w", err)
		}
		slog.Info("migrations applied")
	}
	return gormDB, cleanup, nil
}

// provideTunnelRunner returns the Cloudflare-tunnel starter, or nil when no
// cloudflare_tunnel_token is configured (the default — no tunnel). App.Run
// starts it alongside the listener and cancels it on shutdown.
func provideTunnelRunner(cfg *config.Config) TunnelRunner {
	token := cfg.CloudflareTunnelToken
	if token == "" {
		return nil
	}
	return func(ctx context.Context) error {
		return cloudflaretunnel.RunCloudflareTunnel(ctx, token)
	}
}

func provideIssuer(cfg *config.Config) *auth.Issuer {
	return &auth.Issuer{
		Secret: []byte(cfg.Auth.JWTSecret),
		TTL:    cfg.Auth.AccessTokenTTL,
	}
}

func provideRefreshIssuer(cfg *config.Config, gormDB *gorm.DB) *auth.RefreshIssuer {
	return &auth.RefreshIssuer{
		DB:  gormDB,
		TTL: cfg.Auth.RefreshTokenTTL,
	}
}

// provideAuthPolicy walks protoregistry once at boot and returns the
// procedure -> policy table declared by the proto options.
func provideAuthPolicy() map[string]auth.Policy {
	policy := auth.BuildPolicy()
	slog.Info("auth policy built", "procedures", len(policy))
	return policy
}

func provideLoginLimiter() *auth.LoginLimiter {
	return auth.NewLoginLimiter(5, 60*time.Second)
}

// provideInterceptors is the single auth gate + the audit log, applied to every
// handler.
func provideInterceptors(issuer *auth.Issuer, policy map[string]auth.Policy, gormDB *gorm.DB) connect.HandlerOption {
	return connect.WithInterceptors(
		auth.NewInterceptor(issuer, policy),
		auth.NewAuditInterceptor(gormDB),
	)
}

// provideConnectorService builds the print-connector registry. The Connect
// stream is public + unauthenticated and the auth interceptor is unary-only, so
// only accept connectors when we actually print through them. On an
// internet-facing deploy (Fly) this keeps the stream shut.
func provideConnectorService(cfg *config.Config) *connector.ConnectorService {
	return connector.NewConnectorService(cfg.Connector.Mode == "connector")
}

func provideSaleService(gormDB *gorm.DB, cfg *config.Config, pusher *connector.ConnectorService) *sale.SaleService {
	svc := sale.NewSaleService(gormDB, cfg.Printer)
	svc.SetConnector(cfg.Connector, pusher)
	return svc
}

func provideSettingsService(gormDB *gorm.DB, cfg *config.Config, version buildVersion) *settings.SettingsService {
	svc := settings.NewSettingsService(gormDB)
	svc.SetConnectorMode(cfg.Connector.Mode)
	svc.SetUpdate(string(version), cfg.Update)
	return svc
}

// provideRootHandler mounts the Connect handlers under /api (so the embedded
// SPA can share the same origin — frontend transport baseUrl="/api"), adds the
// /healthz liveness probe, and serves the SPA from everything else.
func provideRootHandler(h Handlers, interceptors connect.HandlerOption) http.Handler {
	apiMux := http.NewServeMux()
	apiMux.Handle(healthifacev1connect.NewHealthServiceHandler(h.Health, interceptors))
	apiMux.Handle(userifacev1connect.NewAuthServiceHandler(h.Auth, interceptors))
	apiMux.Handle(userifacev1connect.NewUserServiceHandler(h.Users, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewSupplierServiceHandler(h.Suppliers, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewPriceAgreementServiceHandler(h.PriceAgreements, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewProductDiscountServiceHandler(h.ProductDiscounts, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewProductPriceTierServiceHandler(h.ProductPriceTiers, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewProductServiceHandler(h.Products, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewBatchServiceHandler(h.Batches, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewStockMovementServiceHandler(h.Stock, interceptors))
	apiMux.Handle(customerifacev1connect.NewCustomerServiceHandler(h.Customers, interceptors))
	apiMux.Handle(posifacev1connect.NewSaleServiceHandler(h.Sales, interceptors))
	apiMux.Handle(connectorifacev1connect.NewConnectorServiceHandler(h.Connectors, interceptors))
	apiMux.Handle(analyticsifacev1connect.NewAnalyticsServiceHandler(h.Analytics, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchaseOrderServiceHandler(h.PurchaseOrders, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchaseReceiptServiceHandler(h.PurchaseReceipts, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchasePaymentServiceHandler(h.PurchasePayments, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchaseReturnServiceHandler(h.PurchaseReturns, interceptors))
	apiMux.Handle(branchifacev1connect.NewBranchServiceHandler(h.Branches, interceptors))
	apiMux.Handle(stocktakeifacev1connect.NewStocktakeServiceHandler(h.Stocktakes, interceptors))
	apiMux.Handle(prescriptionifacev1connect.NewPrescriptionServiceHandler(h.Prescriptions, interceptors))
	apiMux.Handle(warehouseifacev1connect.NewWarehouseServiceHandler(h.Warehouses, interceptors))
	apiMux.Handle(warehouseifacev1connect.NewStockTransferServiceHandler(h.Transfers, interceptors))
	apiMux.Handle(settingsifacev1connect.NewSettingsServiceHandler(h.Settings, interceptors))
	apiMux.Handle(unitifacev1connect.NewUnitServiceHandler(h.Units, interceptors))
	apiMux.Handle(backupifacev1connect.NewBackupServiceHandler(h.Backups, interceptors))

	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", apiMux))
	root.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	root.Handle("/", web.Handler())
	return root
}

func provideHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true) // h2c: HTTP/2 over plain TCP for gRPC/Connect streams

	return &http.Server{
		Addr:      fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:   handler,
		Protocols: &protocols,
	}
}
