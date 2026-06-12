package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/urfave/cli/v3"
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
	"github.com/justmart/backend/internal/license"
	"github.com/justmart/backend/internal/service/analytics"
	authsvc "github.com/justmart/backend/internal/service/auth"
	"github.com/justmart/backend/internal/service/backup"
	"github.com/justmart/backend/internal/service/batch"
	"github.com/justmart/backend/internal/service/branch"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/connector"
	"github.com/justmart/backend/internal/service/customer"
	"github.com/justmart/backend/internal/service/health"
	"github.com/justmart/backend/internal/service/prescription"
	"github.com/justmart/backend/internal/service/product"
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
)

// serve is the default CLI action: boot and run the HTTP server (Connect API +
// embedded SPA). It auto-migrates on boot (config-gated), ensures the bootstrap
// owner, applies the license-driven business mode, and listens.
func serve(_ context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(cmd.String("config"))
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	gormDB, err := db.Open(cfg)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	// Auto-migrate on boot — OFF by default; opt in with `auto_migrate: true`
	// (the turnkey Docker / Windows configs do, so a freshly deployed binary still
	// brings its own schema up to date). Otherwise migrate explicitly via
	// `cmd/server migrate up`.
	if cfg.Database.ShouldAutoMigrate() {
		sqlDB, err := gormDB.DB()
		if err != nil {
			return fmt.Errorf("get sql.DB for migrate: %w", err)
		}
		if err := dbmigrate.Run(sqlDB, cfg.Database.DriverName()); err != nil {
			return fmt.Errorf("auto-migrate: %w", err)
		}
		slog.Info("migrations applied")
	}

	issuer := &auth.Issuer{
		Secret: []byte(cfg.Auth.JWTSecret),
		TTL:    cfg.Auth.AccessTokenTTL,
	}
	refreshIssuer := &auth.RefreshIssuer{
		DB:  gormDB,
		TTL: cfg.Auth.RefreshTokenTTL,
	}

	policy := auth.BuildPolicy()
	slog.Info("auth policy built", "procedures", len(policy))
	interceptors := connect.WithInterceptors(
		auth.NewInterceptor(issuer, policy),
		auth.NewAuditInterceptor(gormDB),
	)

	loginLimiter := auth.NewLoginLimiter(5, 60*time.Second)
	userSvc := user.NewUserService(gormDB)
	authSvc := authsvc.NewAuthService(gormDB, issuer, refreshIssuer, loginLimiter)
	healthSvc := health.NewHealthService(gormDB)
	supplierSvc := supplier.NewSupplierService(gormDB)
	productSvc := product.NewProductService(gormDB)
	batchSvc := batch.NewBatchService(gormDB)
	stockSvc := stock.NewStockService(gormDB)
	customerSvc := customer.NewCustomerService(gormDB)
	connectorSvc := connector.NewConnectorService(cfg.Connector.Token)
	saleSvc := sale.NewSaleService(gormDB, cfg.Printer)
	saleSvc.SetConnector(cfg.Connector, connectorSvc)
	analyticsSvc := analytics.NewAnalyticsService(gormDB)
	purchaseOrdersSvc := purchasing.NewPurchaseOrderService(gormDB)
	purchaseReceiptsSvc := purchasing.NewPurchaseReceiptService(gormDB)
	purchasePaymentsSvc := purchasing.NewPurchasePaymentService(gormDB)
	branchesSvc := branch.NewBranchService(gormDB)
	stocktakesSvc := stocktake.NewStocktakeService(gormDB)
	prescriptionsSvc := prescription.NewPrescriptionService(gormDB)
	warehousesSvc := warehouse.NewWarehouseService(gormDB)
	transfersSvc := transfer.NewTransferService(gormDB)
	settingsSvc := settings.NewSettingsService(gormDB)
	unitsSvc := unit.NewUnitService(gormDB)
	backupSvc := backup.NewBackupService(gormDB, cfg)

	if err := userSvc.EnsureBootstrapOwner(context.Background(), cfg.Bootstrap); err != nil {
		return fmt.Errorf("bootstrap owner: %w", err) // server can't start without it
	}

	// License drives the business mode. Precedence: a config/env license
	// (deployment-provided) wins; otherwise fall back to one applied via the
	// Settings UI (stored in app_settings — SettingsService.ApplyLicense). The
	// effective token is re-verified + its business type re-applied every boot
	// (the license is the source of truth). A config license is also mirrored
	// into storage so the Settings UI reflects it and it survives env-var removal.
	// An invalid/absent license is non-fatal (the shop runs unlicensed / mode 0).
	{
		bootCtx := context.Background()
		token := cfg.License
		source := "config"
		if token == "" {
			token, _ = common.GetLicense(bootCtx, gormDB) // UI-applied fallback
			source = "stored"
		}
		if token != "" {
			if lc, err := license.Verify(token); err != nil {
				slog.Warn("license invalid; business mode left unset", "source", source, "error", err)
			} else {
				// On the config path, apply the mode AND mirror the token+name into
				// storage atomically (so the Settings UI reflects it and it survives
				// env-var removal). On the stored path the token is already saved, so
				// only the mode is re-applied.
				var applyErr error
				if source == "config" {
					applyErr = gormDB.WithContext(bootCtx).Transaction(func(tx *gorm.DB) error {
						if e := common.SetBussinessType(bootCtx, tx, lc.BusinessType); e != nil {
							return e
						}
						return common.SetLicense(bootCtx, tx, token, lc.Name)
					})
				} else {
					applyErr = common.SetBussinessType(bootCtx, gormDB, lc.BusinessType)
				}
				if applyErr != nil {
					slog.Error("license loaded but could not persist", "source", source, "error", applyErr)
				} else {
					slog.Info("license loaded", "name", lc.Name, "business_type", lc.BusinessType, "source", source)
				}
			}
		}
	}

	// Background sweeper: hard-delete abandoned DRAFT carts the POS client missed
	// (crashes, lost sessions). In-process, single-node — like the rate limiter.
	sale.StartDraftSweeper(gormDB)

	// Connect RPC handlers live on apiMux and are mounted under /api so the
	// embedded SPA can share the same origin (frontend transport baseUrl="/api").
	apiMux := http.NewServeMux()
	apiMux.Handle(healthifacev1connect.NewHealthServiceHandler(healthSvc, interceptors))
	apiMux.Handle(userifacev1connect.NewAuthServiceHandler(authSvc, interceptors))
	apiMux.Handle(userifacev1connect.NewUserServiceHandler(userSvc, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewSupplierServiceHandler(supplierSvc, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewProductServiceHandler(productSvc, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewBatchServiceHandler(batchSvc, interceptors))
	apiMux.Handle(inventoryifacev1connect.NewStockMovementServiceHandler(stockSvc, interceptors))
	apiMux.Handle(customerifacev1connect.NewCustomerServiceHandler(customerSvc, interceptors))
	apiMux.Handle(posifacev1connect.NewSaleServiceHandler(saleSvc, interceptors))
	apiMux.Handle(connectorifacev1connect.NewConnectorServiceHandler(connectorSvc, interceptors))
	apiMux.Handle(analyticsifacev1connect.NewAnalyticsServiceHandler(analyticsSvc, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchaseOrderServiceHandler(purchaseOrdersSvc, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchaseReceiptServiceHandler(purchaseReceiptsSvc, interceptors))
	apiMux.Handle(purchasingifacev1connect.NewPurchasePaymentServiceHandler(purchasePaymentsSvc, interceptors))
	apiMux.Handle(branchifacev1connect.NewBranchServiceHandler(branchesSvc, interceptors))
	apiMux.Handle(stocktakeifacev1connect.NewStocktakeServiceHandler(stocktakesSvc, interceptors))
	apiMux.Handle(prescriptionifacev1connect.NewPrescriptionServiceHandler(prescriptionsSvc, interceptors))
	apiMux.Handle(warehouseifacev1connect.NewWarehouseServiceHandler(warehousesSvc, interceptors))
	apiMux.Handle(warehouseifacev1connect.NewStockTransferServiceHandler(transfersSvc, interceptors))
	apiMux.Handle(settingsifacev1connect.NewSettingsServiceHandler(settingsSvc, interceptors))
	apiMux.Handle(unitifacev1connect.NewUnitServiceHandler(unitsSvc, interceptors))
	apiMux.Handle(backupifacev1connect.NewBackupServiceHandler(backupSvc, interceptors))

	// Root mux: /api/* → Connect handlers, /healthz → liveness probe,
	// everything else → the embedded SPA (single self-contained binary).
	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", apiMux))
	root.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	root.Handle("/", web.Handler())

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true) // h2c: HTTP/2 over plain TCP for gRPC/Connect streams

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:      addr,
		Handler:   root,
		Protocols: &protocols,
	}

	slog.Info("justmart listening", "addr", addr)
	return srv.ListenAndServe()
}
