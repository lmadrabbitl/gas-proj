import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';

import { InvestmentsService } from '../../data/investments.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { investmentAssetTypeLabel } from '../../shared/labels';
import { uiMessages } from '../../shared/messages';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { InvestmentPosition, InvestmentPositionQuote } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

@Component({
  selector: 'app-investment-positions',
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
      <a routerLink="/investments/dashboard" routerLinkActive="active">{{ nav.dashboard }}</a>
      <a routerLink="/investments/positions" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }">
        {{ nav.positions }}
      </a>
      <a routerLink="/investments/assets" routerLinkActive="active">{{ nav.assets }}</a>
      <a routerLink="/investments/insert" routerLinkActive="active">{{ nav.insert }}</a>
      <a routerLink="/investments/operations" routerLinkActive="active">{{ nav.operations }}</a>
      <a routerLink="/investments/portfolios" routerLinkActive="active">{{ nav.portfolios }}</a>
    </nav>

    <section class="panel">
      @if (!loading() && positions().length === 0) {
        <p class="state-message">{{ messages.empty }}</p>
      } @else if (!loading()) {
        <div class="table-wrap">
          <table class="positions-table">
            <colgroup>
              <col class="positions-col-ticker" />
              <col class="positions-col-asset" />
              <col class="positions-col-portfolios" />
              <col class="positions-col-type" />
              <col class="positions-col-quantity" />
              <col class="positions-col-price" />
              <col class="positions-col-price" />
              <col class="positions-col-money" />
              <col class="positions-col-money" />
              <col class="positions-col-percent" />
              <col class="positions-col-percent" />
            </colgroup>
            <thead>
              <tr>
                <th>
                  <button class="sort-header-button" type="button" (click)="toggleSort('ticker')">
                    {{ messages.columns.ticker }} <span>{{ sortIndicator('ticker') }}</span>
                  </button>
                </th>
                <th>{{ messages.columns.asset }}</th>
                <th>{{ messages.columns.portfolios }}</th>
                <th>{{ messages.columns.type }}</th>
                <th>
                  <button class="sort-header-button" type="button" (click)="toggleSort('quantity')">
                    {{ messages.columns.quantity }} <span>{{ sortIndicator('quantity') }}</span>
                  </button>
                </th>
                <th>
                  <button class="sort-header-button" type="button" (click)="toggleSort('currentPrice')">
                    {{ messages.columns.currentPrice }} <span>{{ sortIndicator('currentPrice') }}</span>
                  </button>
                </th>
                <th>
                  <button class="sort-header-button" type="button" (click)="toggleSort('averagePrice')">
                    {{ messages.columns.averagePrice }} <span>{{ sortIndicator('averagePrice') }}</span>
                  </button>
                </th>
                <th>
                  <button class="sort-header-button amount-header-button" type="button" (click)="toggleSort('costBasis')">
                    {{ messages.columns.costBasis }} <span>{{ sortIndicator('costBasis') }}</span>
                  </button>
                </th>
                <th>
                  <button class="sort-header-button amount-header-button" type="button" (click)="toggleSort('dividends')">
                    {{ messages.columns.dividends }} <span>{{ sortIndicator('dividends') }}</span>
                  </button>
                </th>
                <th>
                  <button class="sort-header-button amount-header-button" type="button" (click)="toggleSort('result')">
                    {{ messages.columns.realizedPnl }} <span>{{ sortIndicator('result') }}</span>
                  </button>
                </th>
                <th>
                  <button class="sort-header-button amount-header-button" type="button" (click)="toggleSort('resultWithDividends')">
                    {{ messages.columns.totalPnlWithDividends }} <span>{{ sortIndicator('resultWithDividends') }}</span>
                  </button>
                </th>
              </tr>
            </thead>
            <tbody>
              @for (position of sortedPositions(); track position.asset_code) {
                <tr>
                  <td>{{ position.asset_code }}</td>
                  <td>{{ position.asset_name || '—' }}</td>
                  <td>{{ portfolioNamesDisplay(position) }}</td>
                  <td>{{ assetType(position.asset_type) }}</td>
                  <td class="amount-cell">{{ position.current_quantity }}</td>
                  <td class="amount-cell">{{ quoteDisplay(position) }}</td>
                  <td class="amount-cell">{{ money(position.average_price) }}</td>
                  <td class="amount-cell">{{ money(position.total_cost_basis) }}</td>
                  <td class="amount-cell">{{ money(position.matched_dividends_total) }}</td>
                  <td [class.result-positive]="isPositiveResult(position)" [class.result-negative]="isNegativeResult(position)">
                    {{ resultDisplay(position) }}
                  </td>
                  <td [class.result-positive]="isPositiveResultWithDividends(position)" [class.result-negative]="isNegativeResultWithDividends(position)">
                    {{ resultWithDividendsDisplay(position) }}
                  </td>
                </tr>
              }
            </tbody>
          </table>
        </div>
      }
    </section>
  `,
  styles: [`
    .investment-subnav {
      display: flex;
      gap: 12px;
      align-items: center;
      padding: 12px 16px;
      margin-bottom: 18px;
    }

    .investment-subnav a {
      color: var(--muted);
      text-decoration: none;
      padding: 8px 12px;
      border-radius: 999px;
      background: transparent;
    }

    .investment-subnav a.active {
      color: var(--text);
      background: var(--accent-soft);
    }

    .page-subtitle {
      margin: 6px 0 0;
      color: var(--muted);
    }

    table tbody tr:nth-child(even) td {
      background: var(--surface-soft);
    }

    .positions-table {
      table-layout: fixed;
    }

    .positions-col-ticker {
      width: 84px;
    }

    .positions-col-asset {
      width: auto;
    }

    .positions-col-portfolios {
      width: 170px;
    }

    .positions-col-type {
      width: 82px;
    }

    .positions-col-quantity {
      width: 88px;
    }

    .positions-col-price,
    .positions-col-money {
      width: 118px;
    }

    .positions-col-percent {
      width: 132px;
    }

    .result-positive {
      color: #1b7f3b;
      font-weight: 600;
    }

    .result-negative {
      color: #b42318;
      font-weight: 600;
    }

    .sort-header-button {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 0;
      border: 0;
      background: transparent;
      color: inherit;
      font: inherit;
      font-weight: inherit;
      text-transform: uppercase;
      cursor: pointer;
    }

    .amount-header-button {
      justify-content: flex-end;
      width: 100%;
    }
  `],
})
export class InvestmentPositionsComponent implements OnInit {
  private readonly moneyVisibility = inject(MoneyVisibilityService);
  readonly commonMessages = uiMessages.common;
  readonly nav = uiMessages.investments.nav;
  readonly messages = uiMessages.investments.positions;
  readonly loading = signal(true);
  readonly loadingQuotes = signal(false);
  readonly positions = signal<InvestmentPosition[]>([]);
  readonly sortKey = signal<'ticker' | 'quantity' | 'currentPrice' | 'averagePrice' | 'costBasis' | 'dividends' | 'result' | 'resultWithDividends'>('ticker');
  readonly sortDirection = signal<'asc' | 'desc'>('asc');
  readonly sortedPositions = computed(() => {
    const rows = [...this.positions()];
    const sortKey = this.sortKey();
    const direction = this.sortDirection() === 'asc' ? 1 : -1;

    rows.sort((left, right) => {
      if (sortKey === 'ticker') {
        return left.asset_code.localeCompare(right.asset_code) * direction;
      }

      const leftValue = this.sortValue(left, sortKey);
      const rightValue = this.sortValue(right, sortKey);
      if (leftValue === null && rightValue === null) {
        return left.asset_code.localeCompare(right.asset_code);
      }
      if (leftValue === null) {
        return 1;
      }
      if (rightValue === null) {
        return -1;
      }
      if (leftValue === rightValue) {
        return left.asset_code.localeCompare(right.asset_code);
      }
      return (leftValue - rightValue) * direction;
    });

    return rows;
  });

  constructor(
    private readonly investmentsService: InvestmentsService,
    private readonly toast: ToastService,
  ) {}

  ngOnInit(): void {
    this.investmentsService.listPositions().subscribe({
      next: (positions) => {
        this.positions.set(positions);
        this.loading.set(false);
        this.loadQuotes();
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
    });
  }

  assetType(type: InvestmentPosition['asset_type']): string {
    return investmentAssetTypeLabel(type);
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrency(value);
  }

  portfolioNamesDisplay(position: InvestmentPosition): string {
    return position.portfolio_names?.length ? position.portfolio_names.join(', ') : '—';
  }

  toggleSort(key: 'ticker' | 'quantity' | 'currentPrice' | 'averagePrice' | 'costBasis' | 'dividends' | 'result' | 'resultWithDividends'): void {
    if (this.sortKey() === key) {
      this.sortDirection.update((current) => (current === 'asc' ? 'desc' : 'asc'));
      return;
    }
    this.sortKey.set(key);
    this.sortDirection.set(key === 'ticker' ? 'asc' : 'desc');
  }

  sortIndicator(key: 'ticker' | 'quantity' | 'currentPrice' | 'averagePrice' | 'costBasis' | 'dividends' | 'result' | 'resultWithDividends'): string {
    if (this.sortKey() !== key) {
      return this.commonMessages.sortNeutral;
    }
    return this.sortDirection() === 'asc' ? this.commonMessages.sortAsc : this.commonMessages.sortDesc;
  }

  quoteDisplay(position: InvestmentPosition): string {
    if (typeof position.current_price === 'number') {
      return this.moneyVisibility.formatCurrency(position.current_price);
    }
    return this.loadingQuotes() ? this.messages.states.quotePending : '—';
  }

  resultDisplay(position: InvestmentPosition): string {
    const value = this.resultValue(position);
    if (value === null) {
      return this.loadingQuotes() ? this.messages.states.quotePending : '—';
    }
    return `${value.toFixed(2).replace('.', ',')}%`;
  }

  resultWithDividendsDisplay(position: InvestmentPosition): string {
    const value = this.resultWithDividendsValue(position);
    if (value === null) {
      return this.loadingQuotes() ? this.messages.states.quotePending : '—';
    }
    return `${value.toFixed(2).replace('.', ',')}%`;
  }

  resultValue(position: InvestmentPosition): number | null {
    if (typeof position.current_price !== 'number') {
      return null;
    }
    if (position.average_price <= 0) {
      return null;
    }
    return ((position.current_price - position.average_price) / position.average_price) * 100;
  }

  resultWithDividendsValue(position: InvestmentPosition): number | null {
    if (typeof position.current_price !== 'number') {
      return null;
    }
    if (position.total_cost_basis <= 0) {
      return null;
    }

    const currentValue = position.current_price * position.current_quantity;
    return ((currentValue - position.total_cost_basis + position.matched_dividends_total) / position.total_cost_basis) * 100;
  }

  isPositiveResult(position: InvestmentPosition): boolean {
    const value = this.resultValue(position);
    return value !== null && value > 0;
  }

  isNegativeResult(position: InvestmentPosition): boolean {
    const value = this.resultValue(position);
    return value !== null && value < 0;
  }

  isPositiveResultWithDividends(position: InvestmentPosition): boolean {
    const value = this.resultWithDividendsValue(position);
    return value !== null && value > 0;
  }

  isNegativeResultWithDividends(position: InvestmentPosition): boolean {
    const value = this.resultWithDividendsValue(position);
    return value !== null && value < 0;
  }

  private sortValue(
    position: InvestmentPosition,
    key: 'quantity' | 'currentPrice' | 'averagePrice' | 'costBasis' | 'dividends' | 'result' | 'resultWithDividends',
  ): number | null {
    switch (key) {
      case 'quantity':
        return position.current_quantity;
      case 'currentPrice':
        return typeof position.current_price === 'number' ? position.current_price : null;
      case 'averagePrice':
        return position.average_price;
      case 'costBasis':
        return position.total_cost_basis;
      case 'dividends':
        return position.matched_dividends_total;
      case 'result':
        return this.resultValue(position);
      case 'resultWithDividends':
        return this.resultWithDividendsValue(position);
    }
  }

  private loadQuotes(): void {
    if (this.positions().length === 0) {
      return;
    }
    this.loadingQuotes.set(true);
    this.investmentsService.listPositionQuotes().subscribe({
      next: (quotes) => {
        this.applyQuotes(quotes);
        this.loadingQuotes.set(false);
      },
      error: () => {
        this.loadingQuotes.set(false);
      },
    });
  }

  private applyQuotes(quotes: InvestmentPositionQuote[]): void {
    const byCode = new Map(quotes.map((quote) => [quote.asset_code, quote]));
    this.positions.update((rows) =>
      rows.map((row) => {
        const quote = byCode.get(row.asset_code);
        if (!quote) {
          return row;
        }
        return {
          ...row,
          current_price: quote.current_price,
          quote_updated_at: quote.quote_updated_at,
        };
      }),
    );
  }
}
