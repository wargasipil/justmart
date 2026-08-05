import React from "react";
import ReactDOM from "react-dom/client";
import { ChakraProvider, defaultSystem } from "@chakra-ui/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { createBrowserRouter, RouterProvider, Navigate } from "react-router-dom";

import "./lib/i18n";
import { z } from "zod";
import { zodErrorMap } from "./lib/zodErrorMap";

// Install the global Zod error map once at boot (after i18n is configured) so
// every form's validation messages resolve through the translated validation.*
// namespace. Registered here rather than in i18n.ts to avoid an import cycle.
z.setErrorMap(zodErrorMap);

import App from "./App";
import ErrorBoundary from "./components/ErrorBoundary";
import { AuthProvider } from "./lib/auth";
import { queryClient } from "./lib/queryClient";
import { AppToaster } from "./lib/toaster";
import "./stores/preferences"; // applies persisted theme to <html> on boot

import ProtectedRoute from "./components/ProtectedRoute";
import Login from "./routes/Login";
import Dashboard from "./routes/Dashboard";
import Users from "./routes/Users";
import Customers from "./routes/Customers";
import Orders from "./routes/Orders";
import MyPerformance from "./routes/MyPerformance";
import Prescriptions from "./routes/Prescriptions";
import NewPrescription from "./routes/prescriptions/NewPrescription";
import OrderDetail from "./routes/OrderDetail";
import Inventory from "./routes/Inventory";
import Pos from "./routes/Pos";
import Products from "./routes/inventory/Products";
import ProductDetail from "./routes/inventory/ProductDetail";
import Suppliers from "./routes/inventory/Suppliers";
import PriceAgreements from "./routes/inventory/PriceAgreements";
import NewPriceAgreement from "./routes/inventory/NewPriceAgreement";
import SupplierDetail from "./routes/inventory/SupplierDetail";
import Batches from "./routes/inventory/Batches";
import Movements from "./routes/inventory/Movements";
import Stocktake from "./routes/inventory/Stocktake";
import StocktakeDetail from "./routes/inventory/StocktakeDetail";
import Analytics from "./routes/Analytics";
import DailyAnalytics from "./routes/analytics/Daily";
import ProductAnalytics from "./routes/analytics/Product";
import UserAnalytics from "./routes/analytics/User";
import Purchasing from "./routes/purchasing/Purchasing";
import Warehouses from "./routes/Warehouses";
import WarehouseDetail from "./routes/WarehouseDetail";
import Profile from "./routes/Profile";
import Settings from "./routes/Settings";
import SettingsGeneral from "./routes/settings/SettingsGeneral";
import SettingsUnits from "./routes/settings/SettingsUnits";
import SettingsPrinting from "./routes/settings/SettingsPrinting";
import SettingsBackups from "./routes/settings/SettingsBackups";
import SettingsUpdates from "./routes/settings/SettingsUpdates";
import Transfers from "./routes/inventory/Transfers";
import PurchaseOrdersList from "./routes/purchasing/PurchaseOrdersList";
import SuppliersLedger from "./routes/purchasing/SuppliersLedger";
import NewPurchaseOrder from "./routes/purchasing/NewPurchaseOrder";
import PurchaseOrderDetail from "./routes/purchasing/PurchaseOrderDetail";
import { Role } from "./gen/auth_iface/v1/policy_pb";

// Dev-only component gallery (/components). Lazy-loaded and registered only
// under `import.meta.env.DEV`, so a production build never routes to it.
const ComponentGallery = React.lazy(() => import("./routes/dev/Components"));
const devRoutes = import.meta.env.DEV
  ? [
      {
        element: <ProtectedRoute />,
        children: [
          // Both paths hit the same page; it redirects to the first group when
          // :group is missing or unknown.
          { path: "components", element: <ComponentGalleryRoute /> },
          { path: "components/:group", element: <ComponentGalleryRoute /> },
        ],
      },
    ]
  : [];

function ComponentGalleryRoute() {
  return (
    <React.Suspense fallback={null}>
      <ComponentGallery />
    </React.Suspense>
  );
}

const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { path: "login", element: <Login /> },
      ...devRoutes,
      {
        element: <ProtectedRoute />,
        children: [
          { index: true, element: <Dashboard /> },
          { path: "pos", element: <Pos /> },
          // Own account page — every authenticated role has one, so it sits
          // here and not under the OWNER-gated /settings tabs.
          { path: "profile", element: <Profile /> },
        ],
      },
      {
        element: <ProtectedRoute requiredRole={Role.OWNER} />,
        children: [
          { path: "users", element: <Users /> },
          {
            path: "settings",
            element: <Settings />,
            children: [
              { index: true, element: <Navigate to="general" replace /> },
              { path: "general", element: <SettingsGeneral /> },
              { path: "units", element: <SettingsUnits /> },
              { path: "printing", element: <SettingsPrinting /> },
              { path: "backups", element: <SettingsBackups /> },
              { path: "updates", element: <SettingsUpdates /> },
            ],
          },
        ],
      },
      {
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST, Role.CASHIER, Role.APOTEKER]} />,
        children: [
          { path: "customers", element: <Customers /> },
          { path: "orders", element: <Orders /> },
          { path: "orders/:id", element: <OrderDetail /> },
        ],
      },
      {
        // Self-scoped "My performance" — cashier-facing (own revenue/qty over
        // time). OWNER/PHARMACIST have full analytics instead.
        element: <ProtectedRoute requiredRoles={[Role.CASHIER, Role.APOTEKER]} />,
        children: [{ path: "my-performance", element: <MyPerformance /> }],
      },
      {
        // Resep (prescriptions) — pharmacy mode. The Rx authority is OWNER +
        // PHARMACIST + APOTEKER (the licensed pharmacist); CASHIER is excluded.
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST, Role.APOTEKER]} />,
        children: [
          { path: "prescriptions/new", element: <NewPrescription /> },
          { path: "prescriptions", element: <Prescriptions /> },
        ],
      },
      {
        // The catalog is readable by the till: a cashier needs to look up a
        // price, a stock level or an expiry without leaving the app. Both pages
        // render read-only and cost-free for CASHIER/APOTEKER (see
        // lib/roles.ts canSeeCost); the write RPCs stay OWNER+PHARMACIST.
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST, Role.CASHIER, Role.APOTEKER]} />,
        children: [
          { path: "products", element: <Products /> },
          { path: "products/:id", element: <ProductDetail /> },
        ],
      },
      {
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST]} />,
        children: [
          {
            path: "inventory",
            element: <Inventory />,
            children: [
              { index: true, element: <Navigate to="suppliers" replace /> },
              // Moved to the top-level /products route; keep a redirect for old links.
              { path: "products", element: <Navigate to="/products" replace /> },
              { path: "suppliers", element: <Suppliers /> },
              { path: "suppliers/:id", element: <SupplierDetail /> },
              { path: "price-agreements", element: <PriceAgreements /> },
              { path: "price-agreements/new", element: <NewPriceAgreement /> },
              { path: "batches", element: <Batches /> },
              { path: "movements", element: <Movements /> },
              { path: "stocktake", element: <Stocktake /> },
              { path: "stocktake/:id", element: <StocktakeDetail /> },
              { path: "transfers", element: <Transfers /> },
            ],
          },
          {
            path: "analytics",
            element: <Analytics />,
            children: [
              { index: true, element: <Navigate to="daily" replace /> },
              { path: "daily", element: <DailyAnalytics /> },
              { path: "product", element: <ProductAnalytics /> },
              { path: "user", element: <UserAnalytics /> },
              // Back-compat: every URL the old analytics shipped at points here now.
              { path: "operations", element: <Navigate to="/analytics/daily" replace /> },
              { path: "profitability", element: <Navigate to="/analytics/daily" replace /> },
              { path: "inventory", element: <Navigate to="/analytics/daily" replace /> },
              { path: "sales", element: <Navigate to="/analytics/daily" replace /> },
              { path: "margins", element: <Navigate to="/analytics/daily" replace /> },
            ],
          },
          {
            path: "purchasing",
            element: <Purchasing />,
            children: [
              { index: true, element: <Navigate to="all" replace /> },
              // The active status comes from the URL segment via
              // PO_STATUS_BY_TAB, resolved once in the Purchasing shell and
              // shared with the stat row above the tabs — see restockFilters.
              { path: "all", element: <PurchaseOrdersList /> },
              { path: "draft", element: <PurchaseOrdersList /> },
              { path: "sent", element: <PurchaseOrdersList /> },
              { path: "partial", element: <PurchaseOrdersList /> },
              { path: "received", element: <PurchaseOrdersList /> },
              { path: "closed", element: <PurchaseOrdersList /> },
              { path: "voided", element: <PurchaseOrdersList /> },
              { path: "suppliers", element: <SuppliersLedger /> },
              { path: "new", element: <NewPurchaseOrder /> },
              { path: ":id", element: <PurchaseOrderDetail /> },
            ],
          },
          {
            element: <ProtectedRoute requiredRole={Role.OWNER} />,
            children: [
              { path: "warehouses", element: <Warehouses /> },
              { path: "warehouses/:id", element: <WarehouseDetail /> },
            ],
          },
        ],
      },
    ],
  },
], {
  // Opt in early to the router-level v7 behavior (silences its console
  // future-flag warning; no behavioral change for our routes).
  // v7_startTransition is a RouterProvider-level flag (set below), not here.
  future: {
    v7_relativeSplatPath: true,
  },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <ChakraProvider value={defaultSystem}>
          <AuthProvider>
            <RouterProvider router={router} future={{ v7_startTransition: true }} />
            <AppToaster />
          </AuthProvider>
          <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-right" />
        </ChakraProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  </React.StrictMode>,
);
