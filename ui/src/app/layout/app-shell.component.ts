import { Component, signal } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import {
  BadgeDollarSign,
  ChartCandlestick,
  ChartColumn,
  FolderTree,
  LayoutDashboard,
  LucideAngularModule,
  PanelLeftClose,
  PanelLeftOpen,
  Receipt,
  Sparkles,
  SquarePen,
  Wallet,
  LogOut,
  type LucideIconData,
} from 'lucide-angular';

import { AuthService } from '../core/auth.service';
import { ReferenceDataService } from '../data/reference-data.service';
import { UserConfigService } from '../data/user-config.service';
import { getApiErrorMessage } from '../shared/api-error';
import { uiMessages } from '../shared/messages';
import { MoneyVisibilityService } from '../shared/money-visibility.service';
import { ToastService } from '../shared/toast.service';

type NavItem = {
  path: string;
  label: string;
  icon: LucideIconData;
};

@Component({
  selector: 'app-shell',
  imports: [
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
    LucideAngularModule,
  ],
  template: `
    <div
      class="app-shell"
      [class.nav-open]="navOpen()"
      [class.sidebar-collapsed]="sidebarCollapsed()"
    >
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="brand">
            <span class="brand-mark" aria-hidden="true">
              <lucide-icon [img]="brandIcon" [size]="22" [strokeWidth]="2.1" aria-hidden="true" />
            </span>
            <div class="brand-copy">
              <strong>{{ messages.brandName }}</strong>
              <small>{{ messages.brandSubtitle }}</small>
            </div>
          </div>
        </div>

        <div class="sidebar-utility-row">
          <button
            class="icon-button sidebar-toggle-button"
            type="button"
            [attr.aria-label]="sidebarCollapsed() ? messages.expandSidebar : messages.collapseSidebar"
            [title]="sidebarCollapsed() ? messages.expandSidebar : messages.collapseSidebar"
            (click)="toggleSidebarCollapsed()"
          >
            <lucide-icon [img]="sidebarCollapsed() ? panelLeftOpenIcon : panelLeftCloseIcon" [size]="18" [strokeWidth]="1.9" aria-hidden="true" />
          </button>
          <button
            class="icon-button money-visibility-button"
            type="button"
            [attr.aria-label]="
              moneyVisibility.hidden() ? messages.showAmountsAria : messages.hideAmountsAria
            "
            [title]="moneyVisibility.hidden() ? messages.showAmounts : messages.hideAmounts"
            (click)="toggleMoneyVisibility()"
          >
            @if (moneyVisibility.hidden()) {
              <svg aria-hidden="true" viewBox="0 0 24 24">
                <path
                  d="M3.53 2.47 2.47 3.53l3.05 3.05C3.57 8.04 2.19 10 2 12c1.02 4.05 4.68 7 10 7 1.95 0 3.67-.4 5.15-1.11l3.32 3.32 1.06-1.06L3.53 2.47ZM12 17c-4.13 0-6.98-2.45-7.9-5 .16-.49.42-1.03.79-1.58l2.33 2.33A4 4 0 0 0 11.25 16l1.6 1.6c-.28.03-.56.04-.85.04Zm5.95-2.18-2.28-2.28A4 4 0 0 0 9.46 6.3L7.82 4.66A11.8 11.8 0 0 1 12 4c5.32 0 8.98 2.95 10 8-.2.93-.65 1.95-1.35 2.92l-2.7-2.1ZM12 8a4 4 0 0 1 3.99 3.68L12.32 8H12Z"
                />
              </svg>
            } @else {
              <svg aria-hidden="true" viewBox="0 0 24 24">
                <path
                  d="M12 5c5.32 0 8.98 2.95 10 7-1.02 4.05-4.68 7-10 7S3.02 16.05 2 12c1.02-4.05 4.68-7 10-7Zm0 2C7.87 7 5.02 9.45 4.1 12 5.02 14.55 7.87 17 12 17s6.98-2.45 7.9-5C18.98 9.45 16.13 7 12 7Zm0 2.5A2.5 2.5 0 1 1 9.5 12 2.5 2.5 0 0 1 12 9.5Z"
                />
              </svg>
            }
          </button>
        </div>

        <nav class="nav-list" [attr.aria-label]="messages.navAria">
          @for (item of navItems; track item.path) {
            <a
              [routerLink]="item.path"
              routerLinkActive="active"
              [attr.aria-label]="item.label"
              [attr.title]="sidebarCollapsed() ? item.label : null"
            >
              <lucide-icon class="nav-icon" [img]="item.icon" [size]="18" [strokeWidth]="1.9" aria-hidden="true" />
              <span class="nav-label">{{ item.label }}</span>
            </a>
          }
        </nav>

        <button
          class="ghost-button logout-button"
          type="button"
          [attr.title]="sidebarCollapsed() ? messages.logout : null"
          (click)="logout()"
        >
          <lucide-icon class="nav-icon" [img]="logoutIcon" [size]="18" [strokeWidth]="1.9" aria-hidden="true" />
          <span class="nav-label">{{ messages.logout }}</span>
        </button>
      </aside>

      <div class="main-region">
        <header class="topbar">
          <button
            class="icon-button"
            type="button"
            [attr.aria-label]="messages.menuAria"
            (click)="toggleNav()"
          >
            {{ messages.menu }}
          </button>
          <span>{{ messages.topbarTitle }}</span>
          <button
            class="icon-button money-visibility-button topbar-visibility-button"
            type="button"
            [attr.aria-label]="
              moneyVisibility.hidden() ? messages.showAmountsAria : messages.hideAmountsAria
            "
            [title]="moneyVisibility.hidden() ? messages.showAmounts : messages.hideAmounts"
            (click)="toggleMoneyVisibility()"
          >
            @if (moneyVisibility.hidden()) {
              <svg aria-hidden="true" viewBox="0 0 24 24">
                <path
                  d="M3.53 2.47 2.47 3.53l3.05 3.05C3.57 8.04 2.19 10 2 12c1.02 4.05 4.68 7 10 7 1.95 0 3.67-.4 5.15-1.11l3.32 3.32 1.06-1.06L3.53 2.47ZM12 17c-4.13 0-6.98-2.45-7.9-5 .16-.49.42-1.03.79-1.58l2.33 2.33A4 4 0 0 0 11.25 16l1.6 1.6c-.28.03-.56.04-.85.04Zm5.95-2.18-2.28-2.28A4 4 0 0 0 9.46 6.3L7.82 4.66A11.8 11.8 0 0 1 12 4c5.32 0 8.98 2.95 10 8-.2.93-.65 1.95-1.35 2.92l-2.7-2.1ZM12 8a4 4 0 0 1 3.99 3.68L12.32 8H12Z"
                />
              </svg>
            } @else {
              <svg aria-hidden="true" viewBox="0 0 24 24">
                <path
                  d="M12 5c5.32 0 8.98 2.95 10 7-1.02 4.05-4.68 7-10 7S3.02 16.05 2 12c1.02-4.05 4.68-7 10-7Zm0 2C7.87 7 5.02 9.45 4.1 12 5.02 14.55 7.87 17 12 17s6.98-2.45 7.9-5C18.98 9.45 16.13 7 12 7Zm0 2.5A2.5 2.5 0 1 1 9.5 12 2.5 2.5 0 0 1 12 9.5Z"
                />
              </svg>
            }
          </button>
        </header>
        <main class="page">
          <router-outlet />
        </main>
      </div>
    </div>

    @if (toast.current(); as toastMessage) {
      <div class="toast-region" aria-live="polite" aria-atomic="true">
        <div class="toast-card" [class.toast-error]="toastMessage.tone === 'error'" [class.toast-success]="toastMessage.tone === 'success'">
          <span>{{ toastMessage.text }}</span>
          <button class="toast-close" type="button" aria-label="Fechar mensagem" (click)="toast.clear()">×</button>
        </div>
      </div>
    }
  `,
  styles: [`
    .toast-region {
      position: fixed;
      right: 24px;
      bottom: 24px;
      z-index: 1200;
      pointer-events: none;
    }

    .toast-card {
      position: relative;
      display: inline-flex;
      align-items: flex-start;
      gap: 0;
      min-width: 0;
      max-width: min(420px, calc(100vw - 32px));
      padding: 14px 44px 14px 16px;
      border-radius: 18px;
      border: 1px solid color-mix(in srgb, var(--border) 74%, transparent);
      background: color-mix(in srgb, var(--surface-strong) 92%, rgba(255, 255, 255, 0.08));
      box-shadow: 0 18px 42px rgba(15, 23, 42, 0.18);
      color: var(--text);
      backdrop-filter: blur(18px);
      pointer-events: auto;
      animation: toast-enter 180ms ease-out;
    }

    .toast-card span {
      line-height: 1.4;
    }

    .toast-success {
      border-color: color-mix(in srgb, var(--success) 36%, var(--border));
      background: var(--success-bg);
      color: color-mix(in srgb, var(--success) 84%, var(--ink));
    }

    .toast-error {
      border-color: color-mix(in srgb, var(--danger) 40%, var(--border));
      background: var(--danger-bg);
      color: color-mix(in srgb, var(--danger) 82%, var(--ink));
    }

    .toast-close {
      position: absolute;
      top: 10px;
      right: 12px;
      border: none;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font-size: 1.1rem;
      line-height: 1;
      padding: 0;
    }

    @keyframes toast-enter {
      from {
        opacity: 0;
        transform: translateY(10px);
      }
      to {
        opacity: 1;
        transform: translateY(0);
      }
    }

    @media (max-width: 720px) {
      .toast-region {
        right: 16px;
        left: 16px;
        bottom: 16px;
      }

      .toast-card {
        display: flex;
        width: 100%;
        max-width: none;
      }
    }
  `],
})
export class AppShellComponent {
  readonly messages = uiMessages.shell;
  readonly navOpen = signal(false);
  readonly sidebarCollapsed = signal(this.readSidebarCollapsed());
  readonly brandIcon = BadgeDollarSign;
  readonly panelLeftOpenIcon = PanelLeftOpen;
  readonly panelLeftCloseIcon = PanelLeftClose;
  readonly logoutIcon = LogOut;
  readonly navItems: NavItem[] = [
    {
      path: '/dashboard',
      label: this.messages.nav.dashboard,
      icon: LayoutDashboard,
    },
    {
      path: '/insert',
      label: this.messages.nav.insert,
      icon: SquarePen,
    },
    {
      path: '/transactions',
      label: this.messages.nav.transactions,
      icon: Receipt,
    },
    {
      path: '/accounts',
      label: this.messages.nav.accounts,
      icon: Wallet,
    },
    {
      path: '/categories',
      label: this.messages.nav.categories,
      icon: FolderTree,
    },
    {
      path: '/suggestions',
      label: this.messages.nav.suggestions,
      icon: Sparkles,
    },
    {
      path: '/reports',
      label: this.messages.nav.reports,
      icon: ChartColumn,
    },
    {
      path: '/investments',
      label: this.messages.nav.investments,
      icon: ChartCandlestick,
    },
  ];

  constructor(
    private readonly auth: AuthService,
    private readonly referenceData: ReferenceDataService,
    private readonly userConfig: UserConfigService,
    readonly moneyVisibility: MoneyVisibilityService,
    readonly toast: ToastService,
    private readonly router: Router,
  ) {}

  toggleNav(): void {
    this.navOpen.update((value) => !value);
  }

  toggleSidebarCollapsed(): void {
    this.sidebarCollapsed.update((value) => {
      const next = !value;
      this.persistSidebarCollapsed(next);
      return next;
    });
  }

  logout(): void {
    this.auth.logout();
    this.referenceData.clear();
    this.toast.clear();
    this.userConfig.clear();
    void this.router.navigate(['/login']);
  }

  toggleMoneyVisibility(): void {
    const next = !this.moneyVisibility.hidden();
    this.userConfig.updateUIConfig(next).subscribe({
      next: () => {
        this.userConfig.syncUIConfig(next);
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  private readSidebarCollapsed(): boolean {
    if (typeof localStorage === 'undefined') {
      return false;
    }
    return localStorage.getItem('sidebar_collapsed') === 'true';
  }

  private persistSidebarCollapsed(value: boolean): void {
    if (typeof localStorage === 'undefined') {
      return;
    }
    localStorage.setItem('sidebar_collapsed', value ? 'true' : 'false');
  }
}
