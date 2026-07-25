import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';

import { InvestmentsService } from '../../data/investments.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { investmentAssetTypeLabel } from '../../shared/labels';
import { uiMessages } from '../../shared/messages';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { InvestmentAssetType, InvestmentPosition, InvestmentPositionQuote } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

type ValuedPosition = InvestmentPosition & {
  current_value: number;
  unrealized_pnl: number;
};

type AllocationRow = {
  type: InvestmentAssetType;
  value: number;
  percent: number;
};

@Component({
  selector: 'app-investment-dashboard',
  imports: [RouterLink, RouterLinkActive],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.eyebrow }}</p>
        <h1>{{ messages.title }}</h1>
        <p class="page-subtitle">{{ messages.subtitle }}</p>
      </div>
    </section>

    <nav class="panel investment-subnav">
      <a routerLink="/investments/dashboard" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }">{{ nav.dashboard }}</a>
      <a routerLink="/investments/positions" routerLinkActive="active">{{ nav.positions }}</a>
      <a routerLink="/investments/assets" routerLinkActive="active">{{ nav.assets }}</a>
      <a routerLink="/investments/insert" routerLinkActive="active">{{ nav.insert }}</a>
      <a routerLink="/investments/operations" routerLinkActive="active">{{ nav.operations }}</a>
      <a routerLink="/investments/portfolios" routerLinkActive="active">{{ nav.portfolios }}</a>
    </nav>

    @if (loading()) {
      <section class="panel"><p class="state-message">{{ messages.loading }}</p></section>
    } @else if (positions().length === 0) {
      <section class="panel"><p class="state-message">{{ messages.empty }}</p></section>
    } @else {
      @if (loadingQuotes()) {
        <p class="quote-status">{{ messages.quotePending }}</p>
      } @else if (missingQuoteCount() > 0) {
        <p class="quote-status quote-warning">{{ messages.quoteUnavailable }}</p>
      }

      <section class="investment-summary-grid">
        <article class="summary-card primary-summary-card">
          <span>{{ messages.metrics.marketValue }}</span>
          <strong>{{ money(totalMarketValue()) }}</strong>
        </article>
        <article class="summary-card">
          <span>{{ messages.metrics.costBasis }}</span>
          <strong>{{ money(totalCostBasis()) }}</strong>
        </article>
        <article class="summary-card">
          <span>{{ messages.metrics.unrealizedPnl }}</span>
          <strong [class.result-positive]="totalUnrealizedPnl() > 0" [class.result-negative]="totalUnrealizedPnl() < 0">
            {{ signedMoney(totalUnrealizedPnl()) }}
          </strong>
        </article>
        <article class="summary-card">
          <span>{{ messages.metrics.realizedPnl }}</span>
          <strong [class.result-positive]="totalRealizedPnl() > 0" [class.result-negative]="totalRealizedPnl() < 0">
            {{ signedMoney(totalRealizedPnl()) }}
          </strong>
        </article>
        <article class="summary-card">
          <span>{{ messages.metrics.dividends }}</span>
          <strong>{{ money(totalDividends()) }}</strong>
        </article>
        <article class="summary-card">
          <span>{{ messages.metrics.totalPnl }}</span>
          <strong [class.result-positive]="totalPnl() > 0" [class.result-negative]="totalPnl() < 0">
            {{ signedMoney(totalPnl()) }}
          </strong>
        </article>
      </section>

      <section class="dashboard-grid">
        <article class="panel allocation-panel">
          <div class="panel-header">
            <div>
              <h2>{{ messages.allocation.title }}</h2>
              <p>{{ messages.allocation.subtitle }}</p>
            </div>
          </div>
          @if (allocationRows().length === 0) {
            <p class="state-message">{{ messages.allocation.empty }}</p>
          } @else {
            <div class="allocation-list">
              @for (row of allocationRows(); track row.type) {
                <div class="allocation-row">
                  <div class="allocation-label"><span>{{ assetType(row.type) }}</span><strong>{{ row.percent.toFixed(1).replace('.', ',') }}%</strong></div>
                  <div class="allocation-bar"><span [style.width.%]="row.percent"></span></div>
                  <small>{{ money(row.value) }}</small>
                </div>
              }
            </div>
          }
        </article>

        <article class="panel holdings-panel">
          <div class="panel-header">
            <div>
              <h2>{{ messages.holdings.title }}</h2>
              <p>{{ messages.holdings.subtitle }}</p>
            </div>
            <a class="text-link" routerLink="/investments/positions">{{ nav.positions }}</a>
          </div>
          @if (topHoldings().length === 0) {
            <p class="state-message">{{ messages.holdings.empty }}</p>
          } @else {
            <div class="table-wrap">
              <table>
                <thead><tr><th>{{ messages.holdings.ticker }}</th><th>{{ messages.holdings.asset }}</th><th>{{ messages.holdings.type }}</th><th>{{ messages.holdings.quantity }}</th><th>{{ messages.holdings.marketValue }}</th><th>{{ messages.holdings.unrealizedPnl }}</th></tr></thead>
                <tbody>
                  @for (position of topHoldings(); track position.asset_code) {
                    <tr>
                      <td>{{ position.asset_code }}</td>
                      <td>{{ position.asset_name || '—' }}</td>
                      <td>{{ assetType(position.asset_type) }}</td>
                      <td class="amount-cell">{{ position.current_quantity }}</td>
                      <td class="amount-cell">{{ money(position.current_value) }}</td>
                      <td class="amount-cell" [class.result-positive]="position.unrealized_pnl > 0" [class.result-negative]="position.unrealized_pnl < 0">{{ signedMoney(position.unrealized_pnl) }}</td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          }
        </article>
      </section>
    }
  `,
  styles: [`
    .investment-subnav { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; padding: 12px 16px; margin-bottom: 18px; }
    .investment-subnav a { color: var(--muted); text-decoration: none; padding: 8px 12px; border-radius: 999px; }
    .investment-subnav a.active { color: var(--text); background: var(--accent-soft); }
    .page-subtitle, .panel-header p { margin: 6px 0 0; color: var(--muted); }
    .quote-status { margin: 0 0 12px; color: var(--muted); font-size: .9rem; }
    .quote-warning { color: var(--danger); }
    .investment-summary-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-bottom: 18px; }
    .summary-card { display: grid; gap: 8px; min-height: 100px; padding: 18px; border: 1px solid var(--border); border-radius: 16px; background: var(--surface); }
    .summary-card span { color: var(--muted); font-size: .9rem; }
    .summary-card strong { font-size: 1.2rem; }
    .primary-summary-card { background: var(--accent-soft); border-color: color-mix(in srgb, var(--accent) 34%, var(--border)); }
    .dashboard-grid { display: grid; grid-template-columns: minmax(260px, .85fr) minmax(0, 1.7fr); gap: 18px; }
    .panel-header { display: flex; justify-content: space-between; align-items: start; gap: 16px; margin-bottom: 18px; }
    .panel-header h2 { margin: 0; font-size: 1.1rem; }
    .text-link { color: var(--accent-strong); text-decoration: none; white-space: nowrap; }
    .allocation-list { display: grid; gap: 16px; }
    .allocation-row { display: grid; gap: 7px; }
    .allocation-label { display: flex; justify-content: space-between; gap: 12px; }
    .allocation-row small { color: var(--muted); }
    .allocation-bar { height: 9px; border-radius: 999px; overflow: hidden; background: var(--surface-soft); }
    .allocation-bar span { display: block; height: 100%; border-radius: inherit; background: var(--accent); }
    .result-positive { color: #1b7f3b; font-weight: 600; }
    .result-negative { color: #b42318; font-weight: 600; }
    @media (max-width: 980px) { .investment-summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .dashboard-grid { grid-template-columns: 1fr; } }
    @media (max-width: 560px) { .investment-summary-grid { grid-template-columns: 1fr; } }
  `],
})
export class InvestmentDashboardComponent implements OnInit {
  private readonly moneyVisibility = inject(MoneyVisibilityService);
  readonly nav = uiMessages.investments.nav;
  readonly messages = uiMessages.investments.dashboard;
  readonly loading = signal(true);
  readonly loadingQuotes = signal(false);
  readonly positions = signal<InvestmentPosition[]>([]);
  readonly valuedPositions = computed<ValuedPosition[]>(() => this.positions().flatMap((position) => {
    if (typeof position.current_price !== 'number') return [];
    const current_value = position.current_price * position.current_quantity;
    return [{ ...position, current_value, unrealized_pnl: current_value - position.total_cost_basis }];
  }));
  readonly totalMarketValue = computed(() => this.valuedPositions().reduce((total, position) => total + position.current_value, 0));
  readonly totalCostBasis = computed(() => this.positions().reduce((total, position) => total + position.total_cost_basis, 0));
  readonly totalUnrealizedPnl = computed(() => this.valuedPositions().reduce((total, position) => total + position.unrealized_pnl, 0));
  readonly totalRealizedPnl = computed(() => this.positions().reduce((total, position) => total + position.realized_pnl, 0));
  readonly totalDividends = computed(() => this.positions().reduce((total, position) => total + position.matched_dividends_total, 0));
  readonly totalPnl = computed(() => this.totalUnrealizedPnl() + this.totalRealizedPnl() + this.totalDividends());
  readonly missingQuoteCount = computed(() => this.positions().length - this.valuedPositions().length);
  readonly topHoldings = computed(() => [...this.valuedPositions()].sort((a, b) => b.current_value - a.current_value).slice(0, 5));
  readonly allocationRows = computed<AllocationRow[]>(() => {
    const total = this.totalMarketValue();
    if (total <= 0) return [];
    const byType = new Map<InvestmentAssetType, number>();
    for (const position of this.valuedPositions()) byType.set(position.asset_type, (byType.get(position.asset_type) ?? 0) + position.current_value);
    return [...byType.entries()].map(([type, value]) => ({ type, value, percent: (value / total) * 100 })).sort((a, b) => b.value - a.value);
  });

  constructor(private readonly investmentsService: InvestmentsService, private readonly toast: ToastService) {}

  ngOnInit(): void {
    this.investmentsService.listPositions().subscribe({
      next: (positions) => { this.positions.set(positions); this.loading.set(false); this.loadQuotes(); },
      error: (error) => { this.toast.error(getApiErrorMessage(error)); this.loading.set(false); },
    });
  }

  money(value: number): string { return this.moneyVisibility.formatCurrency(value); }
  signedMoney(value: number): string { return this.moneyVisibility.formatSignedCurrency(value); }
  assetType(type: InvestmentAssetType): string { return investmentAssetTypeLabel(type); }

  private loadQuotes(): void {
    if (this.positions().length === 0) return;
    this.loadingQuotes.set(true);
    this.investmentsService.listPositionQuotes().subscribe({
      next: (quotes) => { this.applyQuotes(quotes); this.loadingQuotes.set(false); },
      error: () => this.loadingQuotes.set(false),
    });
  }

  private applyQuotes(quotes: InvestmentPositionQuote[]): void {
    const byCode = new Map(quotes.map((quote) => [quote.asset_code, quote]));
    this.positions.update((positions) => positions.map((position) => {
      const quote = byCode.get(position.asset_code);
      return quote ? { ...position, current_price: quote.current_price, quote_updated_at: quote.quote_updated_at } : position;
    }));
  }
}
