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
import Payroll from "./routes/Payroll";
import Employees from "./routes/payroll/Employees";
import PayrollRuns from "./routes/payroll/PayrollRuns";
import NewPayrollRun from "./routes/payroll/NewPayrollRun";
import PayrollRunDetail from "./routes/payroll/PayrollRunDetail";
import OrderDetail from "./routes/OrderDetail";
import Inventory from "./routes/Inventory";
import Pos from "./routes/Pos";
import Products from "./routes/inventory/Products";
import ProductDetail from "./routes/inventory/ProductDetail";
import Suppliers from "./routes/inventory/Suppliers";
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
import Settings from "./routes/Settings";
import SettingsGeneral from "./routes/settings/SettingsGeneral";
import SettingsUnits from "./routes/settings/SettingsUnits";
import SettingsLicense from "./routes/settings/SettingsLicense";
import SettingsPrinting from "./routes/settings/SettingsPrinting";
import SettingsPayroll from "./routes/settings/SettingsPayroll";
import SettingsIntegrations from "./routes/settings/SettingsIntegrations";
import SettingsGeneralGroup from "./routes/settings/SettingsGeneralGroup";
import SettingsBackups from "./routes/settings/SettingsBackups";
import Transfers from "./routes/inventory/Transfers";
import PurchaseOrdersList from "./routes/purchasing/PurchaseOrdersList";
import SuppliersLedger from "./routes/purchasing/SuppliersLedger";
import NewPurchaseOrder from "./routes/purchasing/NewPurchaseOrder";
import PurchaseOrderDetail from "./routes/purchasing/PurchaseOrderDetail";
import { POStatus } from "./gen/purchasing_iface/v1/order_pb";
import { Role } from "./gen/auth_iface/v1/policy_pb";

const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { path: "login", element: <Login /> },
      {
        element: <ProtectedRoute />,
        children: [
          { index: true, element: <Dashboard /> },
          { path: "pos", element: <Pos /> },
        ],
      },
      {
        element: <ProtectedRoute requiredRole={Role.OWNER} />,
        children: [
          { path: "users", element: <Users /> },
          // Payroll (penggajian) — OWNER only, mode-agnostic. The runs generator +
          // detail are full pages (own BackButton), outside the tab shell.
          {
            path: "payroll",
            element: <Payroll />,
            children: [
              { index: true, element: <Navigate to="employees" replace /> },
              { path: "employees", element: <Employees /> },
              { path: "runs", element: <PayrollRuns /> },
            ],
          },
          { path: "payroll/runs/new", element: <NewPayrollRun /> },
          { path: "payroll/runs/:id", element: <PayrollRunDetail /> },
          {
            path: "settings",
            element: <Settings />,
            children: [
              { index: true, element: <Navigate to="general" replace /> },
              // "General" submenu = a tabbed page; its panels nest under
              // /settings/general/* so the sidebar's prefix-based active/expand
              // highlights "General" on any of its tabs.
              {
                path: "general",
                element: <SettingsGeneralGroup />,
                children: [
                  { index: true, element: <SettingsGeneral /> },
                  { path: "units", element: <SettingsUnits /> },
                  { path: "printing", element: <SettingsPrinting /> },
                  { path: "payroll", element: <SettingsPayroll /> },
                  { path: "backups", element: <SettingsBackups /> },
                ],
              },
              { path: "license", element: <SettingsLicense /> },
              { path: "integrations", element: <SettingsIntegrations /> },
              // Back-compat: old flat URLs redirect to the nested General tabs.
              { path: "units", element: <Navigate to="/settings/general/units" replace /> },
              { path: "printing", element: <Navigate to="/settings/general/printing" replace /> },
              { path: "payroll", element: <Navigate to="/settings/general/payroll" replace /> },
              { path: "backups", element: <Navigate to="/settings/general/backups" replace /> },
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
        element: <ProtectedRoute requiredRoles={[Role.OWNER, Role.PHARMACIST]} />,
        children: [
          { path: "products", element: <Products /> },
          { path: "products/:id", element: <ProductDetail /> },
          {
            path: "inventory",
            element: <Inventory />,
            children: [
              { index: true, element: <Navigate to="suppliers" replace /> },
              // Moved to the top-level /products route; keep a redirect for old links.
              { path: "products", element: <Navigate to="/products" replace /> },
              { path: "suppliers", element: <Suppliers /> },
              { path: "suppliers/:id", element: <SupplierDetail /> },
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
              { path: "all", element: <PurchaseOrdersList /> },
              { path: "draft", element: <PurchaseOrdersList status={POStatus.PO_STATUS_DRAFT} /> },
              { path: "sent", element: <PurchaseOrdersList status={POStatus.PO_STATUS_SENT} /> },
              { path: "partial", element: <PurchaseOrdersList status={POStatus.PO_STATUS_PARTIALLY_RECEIVED} /> },
              { path: "received", element: <PurchaseOrdersList status={POStatus.PO_STATUS_RECEIVED} /> },
              { path: "closed", element: <PurchaseOrdersList status={POStatus.PO_STATUS_CLOSED} /> },
              { path: "voided", element: <PurchaseOrdersList status={POStatus.PO_STATUS_VOIDED} /> },
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
