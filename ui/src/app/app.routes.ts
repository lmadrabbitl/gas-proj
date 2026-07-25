import { Routes } from '@angular/router';

import { authGuard } from './core/auth.guard';
import { guestGuard } from './core/guest.guard';
import { LoginComponent } from './features/login/login.component';
import { AppShellComponent } from './layout/app-shell.component';

export const routes: Routes = [
  {
    path: 'login',
    component: LoginComponent,
    canActivate: [guestGuard],
  },
  {
    path: '',
    component: AppShellComponent,
    canActivate: [authGuard],
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'dashboard' },
      {
        path: 'dashboard',
        loadComponent: () =>
          import('./features/dashboard/dashboard.component').then((m) => m.DashboardComponent),
      },
      {
        path: 'insert',
        loadComponent: () =>
          import('./features/insert/insert-transactions.component').then((m) => m.InsertTransactionsComponent),
      },
      {
        path: 'investments',
        pathMatch: 'full',
        redirectTo: 'investments/dashboard',
      },
      {
        path: 'investments/dashboard',
        loadComponent: () =>
          import('./features/investments/investment-dashboard.component').then((m) => m.InvestmentDashboardComponent),
      },
      {
        path: 'investments/insert',
        loadComponent: () =>
          import('./features/investments/investment-insert.component').then((m) => m.InvestmentInsertComponent),
      },
      {
        path: 'investments/assets',
        loadComponent: () =>
          import('./features/investments/investment-assets.component').then((m) => m.InvestmentAssetsComponent),
      },
      {
        path: 'investments/positions',
        loadComponent: () =>
          import('./features/investments/investment-positions.component').then((m) => m.InvestmentPositionsComponent),
      },
      {
        path: 'investments/operations',
        loadComponent: () =>
          import('./features/investments/investment-operations.component').then((m) => m.InvestmentOperationsComponent),
      },
      {
        path: 'investments/portfolios',
        loadComponent: () =>
          import('./features/investments/investment-portfolios.component').then((m) => m.InvestmentPortfoliosComponent),
      },
      {
        path: 'transactions',
        loadComponent: () =>
          import('./features/transactions/transactions.component').then((m) => m.TransactionsComponent),
      },
      {
        path: 'accounts',
        loadComponent: () =>
          import('./features/accounts/accounts.component').then((m) => m.AccountsComponent),
      },
      {
        path: 'categories',
        loadComponent: () =>
          import('./features/categories/categories.component').then((m) => m.CategoriesComponent),
      },
      {
        path: 'suggestions',
        loadComponent: () =>
          import('./features/suggestions/suggestions.component').then((m) => m.SuggestionsComponent),
      },
      {
        path: 'reports',
        loadComponent: () =>
          import('./features/reports/reports.component').then((m) => m.ReportsComponent),
      },
    ],
  },
  { path: '**', redirectTo: '' },
];
