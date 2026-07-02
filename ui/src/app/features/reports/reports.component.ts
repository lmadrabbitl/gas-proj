import { Component, ElementRef, HostListener, OnInit, signal } from '@angular/core';
import { Router } from '@angular/router';
import { forkJoin } from 'rxjs';

import { ReferenceDataService } from '../../data/reference-data.service';
import { ReportsService } from '../../data/reports.service';
import { UserConfigService } from '../../data/user-config.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { uiMessages } from '../../shared/messages';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { CategoryType, CategoryYearlyBalance, ReportTopItem } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

@Component({
  selector: 'app-reports',
  template: `
    <section class="page-header report-page-header">
      <div>
        <p class="eyebrow">{{ messages.page.eyebrow }}</p>
        <h1>{{ messages.page.title }}</h1>
      </div>
      <div class="report-page-actions">
        <div class="report-year-switcher" [attr.aria-label]="messages.page.yearPickerAria">
          <button class="ghost-button report-year-nav-button" type="button" (click)="changeYear(-1)" [disabled]="loading() || !canGoToPreviousYear()" [attr.aria-label]="messages.page.previousYear" [title]="messages.page.previousYear">&lt;</button>
          <div class="report-year-anchor">
            <button class="ghost-button report-year-pill" type="button" (click)="toggleYearPicker()" [disabled]="loading()">{{ year() }}</button>
            @if (yearPickerOpen()) {
              <div class="report-year-popover">
                <div class="report-year-grid">
                  @for (option of yearOptions(); track option) {
                    <button
                      class="ghost-button report-year-option"
                      type="button"
                      [class.active]="option === year()"
                      (click)="pickYear(option)"
                    >{{ option }}</button>
                  }
                </div>
              </div>
            }
          </div>
          <button class="ghost-button report-year-nav-button" type="button" (click)="changeYear(1)" [disabled]="loading() || !canGoToNextYear()" [attr.aria-label]="messages.page.nextYear" [title]="messages.page.nextYear">&gt;</button>
        </div>
        <button
          class="ghost-button settings-button"
          type="button"
          [title]="messages.page.settingsTitle"
          [attr.aria-label]="messages.page.settingsAria"
          (click)="openSettings()"
        >
          <svg aria-hidden="true" viewBox="0 0 24 24">
            <path
              d="M19.14 12.94c.04-.31.06-.63.06-.94s-.02-.63-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.63l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.08 7.08 0 0 0-1.63-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.49.42l-.36 2.54c-.58.23-1.12.54-1.63.94l-2.39-.96a.5.5 0 0 0-.6.22L2.7 8.85a.5.5 0 0 0 .12.63l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94L2.82 14.52a.5.5 0 0 0-.12.63l1.92 3.32a.5.5 0 0 0 .6.22l2.39-.96c.5.4 1.05.71 1.63.94l.36 2.54a.5.5 0 0 0 .49.42h3.84a.5.5 0 0 0 .5-.42l.36-2.54c.58-.23 1.12-.54 1.63-.94l2.39.96a.5.5 0 0 0 .6-.22l1.92-3.32a.5.5 0 0 0-.12-.63l-2.03-1.58ZM12 15.5A3.5 3.5 0 1 1 12 8.5a3.5 3.5 0 0 1 0 7Z"
            />
          </svg>
        </button>
      </div>
    </section>
    <section class="panel report-panel">
      <div class="panel-header">
        <h2>{{ messages.sections.monthly }}</h2>
      </div>
      @if (loading()) {
        <div class="report-skeleton-stack" data-testid="reports-skeleton">
          @for (sectionTitle of reportSkeletonSections; track sectionTitle) {
            <div class="report-skeleton-section loading-shell">
              <h3 class="report-section-title">{{ sectionTitle }}</h3>
              <div class="table-wrap report-table-wrap">
                <div class="skeleton-table report-skeleton-table">
                  <div class="skeleton-table-header report-skeleton-header">
                    <span class="skeleton skeleton-table-cell header"></span>
                    @for (month of months; track month) {
                      <span class="skeleton skeleton-table-cell header"></span>
                    }
                    <span class="skeleton skeleton-table-cell header"></span>
                  </div>
                  @for (row of reportSkeletonRows; track row) {
                    <div class="skeleton-table-row report-skeleton-row">
                      <span class="skeleton skeleton-table-cell"></span>
                      @for (month of months; track month) {
                        <span class="skeleton skeleton-table-cell"></span>
                      }
                      <span class="skeleton skeleton-table-cell"></span>
                    </div>
                  }
                </div>
              </div>
            </div>
          }
        </div>
      } @else if (yearly().length === 0) {
        <p class="state-message">{{ messages.states.empty }}</p>
      } @else {
        <h3 class="report-section-title">{{ messages.sections.income }}</h3>
        <div class="table-wrap report-table-wrap income-report-table-wrap">
          <table class="report-table income-report-table">
            <thead>
              <tr>
                <th>{{ messages.columns.category }}</th>
                @for (month of months; track month; let monthIndex = $index) {
                  <th [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ month }}</th>
                }
                <th>{{ messages.columns.total }}</th>
              </tr>
            </thead>
            <tbody>
              @for (category of incomeRows(); track category.code) {
                <tr class="parent-report-row">
                  <td>{{ category.name }}</td>
                  @for (amount of category.monthly; track $index; let monthIndex = $index) {
                    <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ reportMoney(amount) }}</td>
                  }
                  <td>{{ reportMoney(rowTotal(category)) }}</td>
                </tr>
                @for (child of category.children; track child.code; let childIndex = $index) {
                  <tr class="sub-row" [class.sub-row-alt]="childIndex % 2 === 1">
                    <td>{{ child.name }}</td>
                    @for (amount of child.monthly; track $index; let monthIndex = $index) {
                      <td
                        [class.current-month-cell]="isCurrentMonthColumn(monthIndex)"
                        [class.report-detail-cell]="cellTopItems(child, monthIndex).length > 0"
                        [attr.tabindex]="cellTopItems(child, monthIndex).length > 0 ? 0 : null"
                        (mouseenter)="positionTooltip($event)"
                        (focusin)="positionTooltip($event)"
                        (mouseleave)="resetTooltipPlacement($event)"
                        (focusout)="resetTooltipPlacement($event)"
                        (click)="openTransactionsForCell(child, monthIndex)"
                        (keydown.enter)="openTransactionsForCell(child, monthIndex)"
                        (keydown.space)="openTransactionsForCell(child, monthIndex); $event.preventDefault()"
                      >
                        <span>{{ reportMoney(amount) }}</span>
                        @if (cellTopItems(child, monthIndex).length > 0) {
                          <div class="report-cell-tooltip">
                            <div class="report-cell-tooltip-title">{{ cellTooltipTitle(child, monthIndex) }}</div>
                            @for (item of cellTopItems(child, monthIndex); track item.description + '-' + item.amount + '-' + $index) {
                              <div class="report-cell-tooltip-row">
                                <span class="report-cell-tooltip-description">{{ item.description }}</span>
                                <span class="report-cell-tooltip-amount">{{ reportMoney(item.amount) }}</span>
                              </div>
                            }
                          </div>
                        }
                      </td>
                    }
                    <td>{{ reportMoney(rowTotal(child)) }}</td>
                  </tr>
                }
              }
              <tr class="total-report-row">
                <td>{{ messages.rows.totalIncome }}</td>
                @for (amount of incomeMonthlyTotals(); track $index; let monthIndex = $index) {
                  <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ reportMoney(amount) }}</td>
                }
                <td>{{ reportMoney(monthlyTotal(incomeMonthlyTotals())) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <h3 class="report-section-title">{{ messages.sections.expenses }}</h3>
        <div class="table-wrap report-table-wrap expense-report-table-wrap">
          <table class="report-table expense-report-table">
            <thead>
              <tr>
                <th>{{ messages.columns.category }}</th>
                @for (month of months; track month; let monthIndex = $index) {
                  <th [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ month }}</th>
                }
                <th>{{ messages.columns.total }}</th>
              </tr>
            </thead>
            <tbody>
              @for (category of expenseRows(); track category.code) {
                <tr class="parent-report-row">
                  <td>{{ category.name }}</td>
                  @for (amount of category.monthly; track $index; let monthIndex = $index) {
                    <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ reportMoney(amount) }}</td>
                  }
                  <td>{{ reportMoney(rowTotal(category)) }}</td>
                </tr>
                @for (child of category.children; track child.code; let childIndex = $index) {
                  <tr class="sub-row" [class.sub-row-alt]="childIndex % 2 === 1">
                    <td>{{ child.name }}</td>
                    @for (amount of child.monthly; track $index; let monthIndex = $index) {
                      <td
                        [class.current-month-cell]="isCurrentMonthColumn(monthIndex)"
                        [class.report-detail-cell]="cellTopItems(child, monthIndex).length > 0"
                        [attr.tabindex]="cellTopItems(child, monthIndex).length > 0 ? 0 : null"
                        (mouseenter)="positionTooltip($event)"
                        (focusin)="positionTooltip($event)"
                        (mouseleave)="resetTooltipPlacement($event)"
                        (focusout)="resetTooltipPlacement($event)"
                        (click)="openTransactionsForCell(child, monthIndex)"
                        (keydown.enter)="openTransactionsForCell(child, monthIndex)"
                        (keydown.space)="openTransactionsForCell(child, monthIndex); $event.preventDefault()"
                      >
                        <span>{{ reportMoney(amount) }}</span>
                        @if (cellTopItems(child, monthIndex).length > 0) {
                          <div class="report-cell-tooltip">
                            <div class="report-cell-tooltip-title">{{ cellTooltipTitle(child, monthIndex) }}</div>
                            @for (item of cellTopItems(child, monthIndex); track item.description + '-' + item.amount + '-' + $index) {
                              <div class="report-cell-tooltip-row">
                                <span class="report-cell-tooltip-description">{{ item.description }}</span>
                                <span class="report-cell-tooltip-amount">{{ reportMoney(item.amount) }}</span>
                              </div>
                            }
                          </div>
                        }
                      </td>
                    }
                    <td>{{ reportMoney(rowTotal(child)) }}</td>
                  </tr>
                }
              }
              <tr class="total-report-row">
                <td>{{ messages.rows.totalExpenses }}</td>
                @for (amount of expenseMonthlyTotals(); track $index; let monthIndex = $index) {
                  <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ reportMoney(amount) }}</td>
                }
                <td>{{ reportMoney(monthlyTotal(expenseMonthlyTotals())) }}</td>
              </tr>
              <tr class="net-report-row">
                <td>{{ messages.rows.net }}</td>
                @for (amount of netMonthlyTotals(); track $index; let monthIndex = $index) {
                  <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)" [class.negative-report-value]="amount < 0">{{ reportMoney(amount) }}</td>
                }
                <td [class.negative-report-value]="monthlyTotal(netMonthlyTotals()) < 0">{{ reportMoney(monthlyTotal(netMonthlyTotals())) }}</td>
              </tr>
              <tr class="accumulated-report-row">
                <td>{{ messages.rows.accumulated }}</td>
                @for (amount of accumulatedMonthlyTotals(); track $index; let monthIndex = $index) {
                  <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)" [class.negative-report-value]="amount < 0">{{ reportMoney(amount) }}</td>
                }
                <td [class.negative-report-value]="lastAccumulatedTotal() < 0">{{ reportMoney(lastAccumulatedTotal()) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <h3 class="report-section-title">{{ messages.sections.summary }}</h3>
        <div class="table-wrap report-table-wrap summary-report-table-wrap">
          <table class="report-table summary-report-table">
            <thead>
              <tr>
                <th>{{ messages.columns.line }}</th>
                @for (month of months; track month; let monthIndex = $index) {
                  <th [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ month }}</th>
                }
                <th>{{ messages.columns.total }}</th>
              </tr>
            </thead>
            <tbody>
              <tr class="summary-revenue-row">
                <td>{{ messages.rows.totalIncome }}</td>
                @for (amount of incomeMonthlyTotals(); track $index; let monthIndex = $index) {
                  <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ reportMoney(amount) }}</td>
                }
                <td>{{ reportMoney(monthlyTotal(incomeMonthlyTotals())) }}</td>
              </tr>
              <tr class="summary-expense-total-row">
                <td>{{ messages.rows.totalExpenses }}</td>
                @for (amount of expenseMonthlyTotals(); track $index; let monthIndex = $index) {
                  <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ reportMoney(amount) }}</td>
                }
                <td>{{ reportMoney(monthlyTotal(expenseMonthlyTotals())) }}</td>
              </tr>
              @for (category of expenseRows(); track category.code) {
                <tr class="summary-expense-row">
                  <td>{{ category.name }}</td>
                  @for (amount of category.monthly; track $index; let monthIndex = $index) {
                    <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)">{{ reportMoney(amount) }}</td>
                  }
                  <td>{{ reportMoney(rowTotal(category)) }}</td>
                </tr>
              }
              <tr class="accumulated-report-row">
                <td>{{ messages.rows.accumulated }}</td>
                @for (amount of accumulatedMonthlyTotals(); track $index; let monthIndex = $index) {
                  <td [class.current-month-cell]="isCurrentMonthColumn(monthIndex)" [class.negative-report-value]="amount < 0">{{ reportMoney(amount) }}</td>
                }
                <td [class.negative-report-value]="lastAccumulatedTotal() < 0">{{ reportMoney(lastAccumulatedTotal()) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      }
    </section>

    @if (settingsPanelOpen()) {
      <aside class="side-panel">
        <div class="panel-header">
          <div>
            <h2>{{ messages.settings.title }}</h2>
            <p>{{ messages.settings.description }}</p>
          </div>
          <button class="ghost-button" type="button" (click)="closeSettings()">{{ messages.page.close }}</button>
        </div>
        <div class="form-stack">
          <label class="checkbox-label">
            <input
              type="checkbox"
              [checked]="pendingShowEmptyCategories()"
              (change)="pendingShowEmptyCategories.set($any($event.target).checked)"
            />
            <span>{{ messages.settings.showEmptyCategories }}</span>
          </label>
          <p class="field-hint">{{ messages.settings.showEmptyCategoriesHint }}</p>
          <button class="primary-button" type="button" (click)="saveSettings()">{{ messages.page.save }}</button>
        </div>
      </aside>
    }
  `,
  styles: [`
    .field-hint {
      color: var(--muted);
      display: block;
      font-size: 0.82rem;
      line-height: 1.45;
      margin-top: 6px;
    }

    .report-page-actions {
      align-items: center;
      display: flex;
      gap: 12px;
      justify-content: flex-end;
      flex-wrap: wrap;
    }

    .report-page-header,
    .report-panel {
      max-width: none;
    }

    .report-table-wrap {
      margin-inline: -12px;
      border-left: 1px solid rgba(198, 209, 203, 0.72);
      border-right: 1px solid rgba(198, 209, 203, 0.72);
      border-radius: 10px;
      box-shadow: none;
      padding-bottom: 0;
    }

    .report-table {
      min-width: 1120px;
      font-size: 0.72rem;
      font-family: Arial, "Helvetica Neue", sans-serif;
    }

    .report-table th,
    .report-table td {
      padding: 5px 6px;
    }

    .report-table .current-month-cell {
      box-shadow: inset 0 0 0 999px var(--report-current-month-overlay);
    }

    .report-table th.current-month-cell {
      background: var(--report-current-month-header);
    }

    .report-table th:first-child,
    .report-table td:first-child {
      min-width: 148px;
      max-width: 176px;
    }

    .report-table th:not(:first-child),
    .report-table td:not(:first-child) {
      min-width: 72px;
    }

    .report-table th:last-child {
      border-left: 1px solid var(--border-strong);
    }

    .report-table td:last-child {
      border-left: 1px solid var(--border);
      font-weight: 800;
    }

    .report-skeleton-stack {
      display: grid;
      gap: 22px;
      padding: 0 0 12px;
    }

    .settings-button {
      align-items: center;
      display: inline-flex;
      justify-content: center;
      min-height: 44px;
      min-width: 44px;
      padding: 0;
    }

    .settings-button svg {
      fill: currentColor;
      height: 18px;
      width: 18px;
    }

    .income-report-table .parent-report-row td {
      background: var(--report-row-parent);
    }

    .income-report-table .current-month-cell {
      box-shadow: inset 0 0 0 999px var(--report-current-month-overlay);
    }

    .income-report-table th.current-month-cell {
      background: var(--report-current-month-header);
    }

    .income-report-table .total-report-row td {
      background: #eef6f1;
    }

    .income-report-table tr > th:last-child,
    .income-report-table tr > td:last-child {
      background: #eef6f1;
    }

    .expense-report-table .parent-report-row td {
      background: #f8ece7;
    }

    .expense-report-table .sub-row td {
      background: #fdf6f3;
    }

    .expense-report-table .sub-row.sub-row-alt td {
      background: #f9eeea;
    }

    .expense-report-table .current-month-cell {
      box-shadow: inset 0 0 0 999px rgba(208, 99, 79, 0.16);
    }

    .expense-report-table th.current-month-cell {
      background: #f5ddd8;
    }

    .expense-report-table .total-report-row td {
      background: #f7e7e4;
    }

    .expense-report-table tr > th:last-child,
    .expense-report-table tr > td:last-child {
      background: #f7e7e4;
    }

    .summary-report-table .current-month-cell {
      box-shadow: inset 0 0 0 999px rgba(214, 170, 78, 0.15);
    }

    .summary-report-table th.current-month-cell {
      background: #f5e7bd;
    }

    .summary-report-table .summary-revenue-row td {
      background: #f7efcf;
    }

    .summary-report-table .summary-expense-total-row td {
      background: #f3e2b6;
    }

    .summary-report-table .summary-expense-row td {
      background: #fbf2d9;
    }

    .summary-report-table .accumulated-report-row td {
      background: #efe5bf;
    }

    .summary-report-table tr > th:last-child,
    .summary-report-table tr > td:last-child {
      background: #f3e2b6;
    }

    .report-skeleton-section {
      display: grid;
      gap: 14px;
      overflow: visible;
    }

    .report-skeleton-table {
      gap: 10px;
    }

    .report-skeleton-header,
    .report-skeleton-row {
      grid-template-columns: minmax(148px, 2fr) repeat(12, minmax(52px, 0.8fr)) minmax(72px, 0.9fr);
      gap: 10px;
    }

    @media (max-width: 900px) {
      .report-page-actions {
        width: 100%;
        justify-content: space-between;
      }

      .report-table-wrap {
        margin-inline: -8px;
      }
    }
  `],
})
export class ReportsComponent implements OnInit {
  readonly messages = uiMessages.reports;
  readonly months = uiMessages.labels.monthsShort;
  readonly minYear = 2020;
  readonly year = signal(new Date().getFullYear());
  readonly loading = signal(true);
  readonly error = signal('');
  readonly yearly = signal<CategoryYearlyBalance[]>([]);
  readonly yearPickerOpen = signal(false);
  readonly settingsPanelOpen = signal(false);
  readonly pendingShowEmptyCategories = signal(true);
  readonly reportSkeletonSections = [
    this.messages.sections.income,
    this.messages.sections.expenses,
    this.messages.sections.summary,
  ];
  readonly reportSkeletonRows = [0, 1, 2, 3, 4, 5];
  private readonly now = new Date();

  constructor(
    private readonly elementRef: ElementRef<HTMLElement>,
    private readonly reports: ReportsService,
    private readonly referenceData: ReferenceDataService,
    private readonly moneyVisibility: MoneyVisibilityService,
    private readonly userConfigService: UserConfigService,
    private readonly toast: ToastService,
    private readonly router: Router,
  ) {}

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent): void {
    if (!this.yearPickerOpen() && !this.settingsPanelOpen()) {
      return;
    }
    if (!this.elementRef.nativeElement.contains(event.target as Node)) {
      this.yearPickerOpen.set(false);
      this.settingsPanelOpen.set(false);
    }
  }

  ngOnInit(): void {
    this.pendingShowEmptyCategories.set(this.showEmptyCategories());
    this.load();
  }

  load(): void {
    this.loading.set(true);
    forkJoin({
      referenceData: this.referenceData.load(),
      yearly: this.reports.yearly(this.year()),
    }).subscribe({
      next: ({ yearly }) => {
        this.yearly.set(yearly);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }

  rowTotal(row: ReportRow): number {
    return row.monthly.reduce((total, amount) => total + amount, 0);
  }

  monthlyTotal(monthly: number[]): number {
    return monthly.reduce((total, amount) => total + amount, 0);
  }

  reportMoney(value: number): string {
    return this.moneyVisibility.formatCompactCurrencyAbsolute(value);
  }

  cellTopItems(row: ReportRow, monthIndex: number): ReportTopItem[] {
    return row.topItemsByMonth?.[monthIndex] ?? [];
  }

  cellTooltipTitle(row: ReportRow, monthIndex: number): string {
    return `Top 5 ${row.name} · ${this.formatReportTooltipMonth(monthIndex)}`;
  }

  openTransactionsForCell(row: ReportRow, monthIndex: number): void {
    if (this.cellTopItems(row, monthIndex).length === 0) {
      return;
    }

    this.router.navigate(['/transactions'], {
      queryParams: {
        category_code: row.code,
        from_date: this.monthQueryDate(monthIndex, 1),
        to_date: this.monthQueryDate(monthIndex, this.lastDayOfMonth(monthIndex)),
      },
    });
  }

  positionTooltip(event: Event): void {
    const cell = event.currentTarget as HTMLElement | null;
    if (!cell?.classList.contains('report-detail-cell')) {
      return;
    }

    requestAnimationFrame(() => this.applyTooltipPlacement(cell));
  }

  resetTooltipPlacement(event: Event): void {
    const cell = event.currentTarget as HTMLElement | null;
    if (!cell) {
      return;
    }

    cell.classList.remove(
      'report-detail-cell-below',
      'report-detail-cell-align-right',
      'report-detail-cell-align-left',
    );
  }

  categoryName(code: string): string {
    return this.referenceData.categoryName(code);
  }

  incomeRows(): ReportRow[] {
    return this.rowsForType('INCOME');
  }

  expenseRows(): ReportRow[] {
    return this.rowsForType('EXPENSE');
  }

  showEmptyCategories(): boolean {
    return this.userConfigService.reportsConfig().show_empty_categories;
  }

  incomeMonthlyTotals(): number[] {
    return this.sumMonthly(this.incomeRows().map((row) => row.monthly));
  }

  expenseMonthlyTotals(): number[] {
    return this.sumMonthly(this.expenseRows().map((row) => row.monthly));
  }

  netMonthlyTotals(): number[] {
    const income = this.incomeMonthlyTotals();
    const expense = this.expenseMonthlyTotals();
    return income.map((amount, index) => amount + (expense[index] ?? 0));
  }

  accumulatedMonthlyTotals(): number[] {
    let accumulated = 0;
    return this.netMonthlyTotals().map((amount) => {
      accumulated += amount;
      return accumulated;
    });
  }

  lastAccumulatedTotal(): number {
    const totals = this.accumulatedMonthlyTotals();
    return totals[totals.length - 1] ?? 0;
  }

  changeYear(delta: number): void {
    const nextYear = Math.min(this.maxYear(), Math.max(this.minYear, this.year() + delta));
    if (nextYear === this.year()) {
      return;
    }
    this.year.set(nextYear);
    this.yearPickerOpen.set(false);
    this.load();
  }

  toggleYearPicker(): void {
    this.yearPickerOpen.update((value) => !value);
  }

  openSettings(): void {
    this.pendingShowEmptyCategories.set(this.showEmptyCategories());
    this.settingsPanelOpen.set(true);
  }

  closeSettings(): void {
    this.settingsPanelOpen.set(false);
  }

  saveSettings(): void {
    const showEmptyCategories = this.pendingShowEmptyCategories();
    this.userConfigService.updateReportsConfig({ show_empty_categories: showEmptyCategories }).subscribe({
      next: () => {
        this.userConfigService.syncReportsConfig({ show_empty_categories: showEmptyCategories });
        this.closeSettings();
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  pickYear(year: number): void {
    if (year === this.year()) {
      this.yearPickerOpen.set(false);
      return;
    }
    this.year.set(year);
    this.yearPickerOpen.set(false);
    this.load();
  }

  yearOptions(): number[] {
    return Array.from({ length: this.maxYear() - this.minYear + 1 }, (_, index) => this.maxYear() - index);
  }

  canGoToPreviousYear(): boolean {
    return this.year() > this.minYear;
  }

  canGoToNextYear(): boolean {
    return this.year() < this.maxYear();
  }

  isCurrentMonthColumn(monthIndex: number): boolean {
    return this.year() === this.now.getFullYear() && monthIndex === this.now.getMonth();
  }

  private rowsForType(type: CategoryType): ReportRow[] {
    return this.yearly()
      .map((category) => this.categoryToReportRow(category, type))
      .filter((row): row is ReportRow => row !== null)
      .filter((row) => this.showEmptyCategories() || this.reportRowHasAnyVisibleValue(row));
  }

  private categoryToReportRow(category: CategoryYearlyBalance, type: CategoryType): ReportRow | null {
    const children = (category.subcategories ?? [])
      .filter((child) => this.categoryType(child.code) === type)
      .map((child) => ({
        code: child.code,
        name: this.categoryName(child.code),
        monthly: child.monthly_data,
        children: [],
        topItemsByMonth: child.top_items_by_month,
      }))
      .filter((child) => this.showEmptyCategories() || this.rowHasAnyValue(child.monthly));

    if (this.categoryType(category.code) !== type) {
      return null;
    }

    return {
      code: category.code,
      name: this.categoryName(category.code),
      monthly: category.monthly_data,
      children,
    };
  }

  private categoryType(code: string): CategoryType | null {
    return this.referenceData.flatCategories().find((category) => category.Code === code)?.Type ?? null;
  }

  private sumMonthly(rows: number[][]): number[] {
    return Array.from({ length: 12 }, (_, monthIndex) =>
      rows.reduce((total, monthly) => total + (monthly[monthIndex] ?? 0), 0),
    );
  }

  private maxYear(): number {
    return this.now.getFullYear();
  }

  private formatReportTooltipMonth(monthIndex: number): string {
    const locale = this.userConfigService.config().language || 'pt-BR';
    const date = new Date(this.year(), monthIndex, 1);
    const month = new Intl.DateTimeFormat(locale, { month: 'long' }).format(date);
    const year = new Intl.DateTimeFormat(locale, { year: '2-digit' }).format(date);
    return `${capitalize(month)}/${year}`;
  }

  private monthQueryDate(monthIndex: number, day: number): string {
    const month = String(monthIndex + 1).padStart(2, '0');
    return `${this.year()}-${month}-${String(day).padStart(2, '0')}`;
  }

  private lastDayOfMonth(monthIndex: number): number {
    return new Date(this.year(), monthIndex + 1, 0).getDate();
  }

  private applyTooltipPlacement(cell: HTMLElement): void {
    const tooltip = cell.querySelector<HTMLElement>('.report-cell-tooltip');
    if (!tooltip) {
      return;
    }

    this.resetTooltipClasses(cell);

    const boundary = cell.closest<HTMLElement>('.report-table-wrap') ?? this.elementRef.nativeElement;
    const boundaryRect = boundary.getBoundingClientRect();
    const cellRect = cell.getBoundingClientRect();
    const tooltipWidth = Math.max(tooltip.offsetWidth, tooltip.scrollWidth);
    const tooltipHeight = Math.max(tooltip.offsetHeight, tooltip.scrollHeight);
    const padding = 8;

    const centeredLeft = cellRect.left + (cellRect.width / 2) - (tooltipWidth / 2);
    const centeredRight = centeredLeft + tooltipWidth;
    if (centeredRight > boundaryRect.right - padding) {
      cell.classList.add('report-detail-cell-align-right');
    } else if (centeredLeft < boundaryRect.left + padding) {
      cell.classList.add('report-detail-cell-align-left');
    }

    const spaceAbove = cellRect.top - boundaryRect.top;
    const spaceBelow = boundaryRect.bottom - cellRect.bottom;
    if (spaceAbove < tooltipHeight + 12 && spaceBelow > spaceAbove) {
      cell.classList.add('report-detail-cell-below');
    }
  }

  private resetTooltipClasses(cell: HTMLElement): void {
    cell.classList.remove(
      'report-detail-cell-below',
      'report-detail-cell-align-right',
      'report-detail-cell-align-left',
    );
  }

  private rowHasAnyValue(monthly: number[]): boolean {
    return monthly.some((amount) => amount !== 0);
  }

  private reportRowHasAnyVisibleValue(row: ReportRow): boolean {
    if (row.children.length > 0) {
      return row.children.length > 0;
    }
    return this.rowHasAnyValue(row.monthly);
  }
}

interface ReportRow {
  code: string;
  name: string;
  monthly: number[];
  children: ReportRow[];
  topItemsByMonth?: Array<ReportTopItem[] | null>;
}

function capitalize(value: string): string {
  if (!value) {
    return value;
  }
  return value.charAt(0).toLocaleUpperCase() + value.slice(1);
}
