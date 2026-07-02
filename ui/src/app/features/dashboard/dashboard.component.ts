import { Component, ElementRef, HostListener, OnInit, computed, signal } from '@angular/core';
import { forkJoin } from 'rxjs';

import { ReferenceDataService } from '../../data/reference-data.service';
import { ReportsService } from '../../data/reports.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { uiMessages } from '../../shared/messages';
import { toBrazilianDate } from '../../shared/money';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { Account, AccountBalance, CategoryType, CategoryYearlyBalance, Transaction } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

@Component({
  selector: 'app-dashboard',
  template: `
    <section class="page-header dashboard-header">
      <div class="dashboard-heading">
        <p class="eyebrow">{{ messages.eyebrow }}</p>
        <h1>{{ messages.title }}</h1>
        <p class="dashboard-subtitle">{{ messages.subtitle }}</p>
      </div>
      <div class="dashboard-actions">
        <div class="dashboard-period-switcher">
          <button class="ghost-button dashboard-nav-button" type="button" [attr.aria-label]="messages.previousMonth" [title]="messages.previousMonth" (click)="goToPreviousMonth()" [disabled]="loading()">&lt;</button>
          <div class="dashboard-period-anchor">
            <button class="ghost-button dashboard-period-pill" type="button" (click)="togglePeriodPicker()" [disabled]="loading()">{{ periodLabel() }}</button>
            @if (periodPickerOpen()) {
              <div class="dashboard-period-popover">
                <label class="dashboard-period-year">
                  <span>{{ messages.chooseYear }}</span>
                  <select [value]="pickerYear()" (change)="onPickerYearChange($any($event.target).value)">
                    @for (year of yearOptions(); track year) {
                      <option [value]="year" [selected]="year === pickerYear()">{{ year }}</option>
                    }
                  </select>
                </label>
                <div class="dashboard-period-months">
                  @for (month of months; track month; let monthIndex = $index) {
                    <button
                      class="ghost-button dashboard-period-month"
                      type="button"
                      [class.active]="pickerYear() === currentYear() && monthIndex + 1 === currentMonth()"
                      [disabled]="isFutureMonth(pickerYear(), monthIndex + 1)"
                      (click)="pickMonth(pickerYear(), monthIndex + 1)"
                    >{{ month }}</button>
                  }
                </div>
              </div>
            }
          </div>
          <button class="ghost-button dashboard-nav-button" type="button" [attr.aria-label]="messages.nextMonth" [title]="messages.nextMonth" (click)="goToNextMonth()" [disabled]="loading() || !canGoToNextMonth()">&gt;</button>
        </div>
        <button class="ghost-button dashboard-refresh-button" type="button" (click)="load()" [disabled]="loading()">{{ messages.refresh }}</button>
      </div>
    </section>
    @if (loading()) {
      <section class="dashboard-summary panel-surface loading-shell dashboard-skeleton" data-testid="dashboard-skeleton">
        <dl class="dashboard-summary-totals">
          <div class="skeleton-card">
            <span class="skeleton skeleton-line short"></span>
            <span class="skeleton skeleton-line long" style="height: 34px;"></span>
            <span class="skeleton skeleton-line medium"></span>
          </div>
        </dl>
        <div class="dashboard-month-strip">
          @for (item of dashboardMetricSkeletonItems; track item) {
            <article class="month-strip-item skeleton-card">
              <span class="skeleton skeleton-line short"></span>
              <span class="skeleton skeleton-line medium" style="height: 26px;"></span>
              <span class="skeleton skeleton-line long"></span>
            </article>
          }
        </div>
      </section>

      <section class="dashboard-workspace dashboard-skeleton">
        <div class="dashboard-main-column">
          <article class="panel panel-surface loading-shell">
            <div class="panel-header">
              <div class="skeleton-card">
                <span class="skeleton skeleton-line medium"></span>
                <span class="skeleton skeleton-line long"></span>
              </div>
            </div>
            <div class="chart-card">
              <span class="skeleton skeleton-chart"></span>
            </div>
          </article>

          <article class="panel panel-surface loading-shell">
            <div class="panel-header">
              <div class="skeleton-card">
                <span class="skeleton skeleton-line medium"></span>
                <span class="skeleton skeleton-line long"></span>
              </div>
            </div>
            <div class="transaction-list skeleton-stack">
              @for (item of dashboardListSkeletonItems; track item) {
                <div class="transaction-row skeleton-row inline">
                  <div class="transaction-copy skeleton-card">
                    <span class="skeleton skeleton-line medium"></span>
                    <span class="skeleton skeleton-line long"></span>
                  </div>
                  <span class="skeleton skeleton-line short" style="width: 92px;"></span>
                </div>
              }
            </div>
          </article>

          <article class="panel panel-surface loading-shell">
            <div class="panel-header category-trends-header">
              <div class="skeleton-card">
                <span class="skeleton skeleton-line medium"></span>
                <span class="skeleton skeleton-line long"></span>
              </div>
            </div>
            <div class="chart-card">
              <span class="skeleton skeleton-chart"></span>
            </div>
          </article>
        </div>

        <aside class="dashboard-side-column">
          <article class="panel panel-surface loading-shell">
            <div class="panel-header">
              <div class="skeleton-card">
                <span class="skeleton skeleton-line medium"></span>
                <span class="skeleton skeleton-line long"></span>
              </div>
            </div>
            <div class="insights-list skeleton-stack">
              <div class="insights-summary">
                @for (item of dashboardInsightSkeletonItems; track item) {
                  <div class="insight-metric skeleton-card">
                    <span class="skeleton skeleton-line short"></span>
                    <span class="skeleton skeleton-line long"></span>
                  </div>
                }
              </div>
              @for (item of dashboardListSkeletonItems; track item) {
                <div class="insight-row">
                  <span class="skeleton skeleton-line short"></span>
                  <span class="skeleton skeleton-line long"></span>
                </div>
              }
            </div>
          </article>

          <article class="panel panel-surface loading-shell">
            <div class="panel-header">
              <div class="skeleton-card">
                <span class="skeleton skeleton-line medium"></span>
                <span class="skeleton skeleton-line long"></span>
              </div>
            </div>
            <div class="donut-layout">
              <div class="donut-wrap">
                <span class="skeleton skeleton-circle" style="width: 180px; height: 180px; margin: 0 auto;"></span>
              </div>
              <div class="breakdown-list">
                @for (item of dashboardBalanceSkeletonItems; track item) {
                  <div class="breakdown-row">
                    <div class="breakdown-meta" style="flex: 1 1 auto;">
                      <span class="skeleton skeleton-line medium"></span>
                    </div>
                    <span class="skeleton skeleton-line short" style="width: 72px;"></span>
                  </div>
                }
              </div>
            </div>
          </article>

          <article class="panel panel-surface loading-shell">
            <div class="panel-header">
              <div class="skeleton-card">
                <span class="skeleton skeleton-line medium"></span>
                <span class="skeleton skeleton-line long"></span>
              </div>
            </div>
            <div class="spotlight-total skeleton-card">
              <span class="skeleton skeleton-line short"></span>
              <span class="skeleton skeleton-line medium" style="height: 28px;"></span>
            </div>
            <div class="mini-chart">
              <span class="skeleton skeleton-chart compact"></span>
            </div>
          </article>

          <article class="panel panel-surface loading-shell">
            <div class="panel-header">
              <div class="skeleton-card">
                <span class="skeleton skeleton-line medium"></span>
                <span class="skeleton skeleton-line long"></span>
              </div>
            </div>
            <div class="category-benchmark-card skeleton-stack">
              @for (item of dashboardListSkeletonItems; track item) {
                <div class="benchmark-row benchmark-row-skeleton">
                  <div class="benchmark-copy skeleton-card">
                    <span class="skeleton skeleton-line medium"></span>
                    <span class="skeleton skeleton-line long"></span>
                  </div>
                  <span class="skeleton skeleton-line short" style="width: 72px;"></span>
                </div>
              }
            </div>
          </article>

          <article class="panel panel-surface loading-shell">
            <div class="panel-header">
              <div class="skeleton-card">
                <span class="skeleton skeleton-line medium"></span>
                <span class="skeleton skeleton-line long"></span>
              </div>
            </div>
            <div class="balance-bars skeleton-stack">
              @for (item of dashboardBalanceSkeletonItems; track item) {
                <div class="balance-row skeleton-row">
                  <div class="balance-row-header">
                    <span class="skeleton skeleton-line medium"></span>
                    <span class="skeleton skeleton-line short" style="width: 84px;"></span>
                  </div>
                  <span class="skeleton skeleton-block" style="height: 10px;"></span>
                </div>
              }
            </div>
          </article>
        </aside>
      </section>
    } @else {
      <section class="dashboard-summary panel-surface">
        <dl class="dashboard-summary-totals">
          <div>
            <dt>{{ messages.hero.netWorth }}</dt>
            <dd [class.negative-value]="totalBalance() < 0">{{ money(totalBalance()) }}</dd>
            <small class="summary-account-breakdown">
              <span>{{ assetAccountsLabel() }}: <b>{{ moneyAbsolute(totalAssetsBalance()) }}</b></span>
              <span>{{ liabilityAccountsLabel() }}: <b>{{ moneyAbsolute(totalLiabilitiesBalance()) }}</b></span>
            </small>
          </div>
        </dl>
        <div class="dashboard-month-strip">
          <article class="month-strip-item">
            <span>{{ messages.hero.highlight }}</span>
            <strong [class.negative-value]="currentMonthNet() < 0">{{ signedMoney(currentMonthNet()) }}</strong>
            <small class="month-comparison" [class.positive-value]="monthlyComparisonDelta() > 0" [class.negative-value]="monthlyComparisonDelta() < 0">
              @if (monthlyComparisonDelta() > 0) {
                <svg aria-hidden="true" viewBox="0 0 16 16" class="trend-icon">
                  <path d="M2 11.5 6.1 7.4l2.6 2.6L14 4.7"></path>
                  <path d="M10.5 4.7H14v3.5"></path>
                </svg>
              } @else if (monthlyComparisonDelta() < 0) {
                <svg aria-hidden="true" viewBox="0 0 16 16" class="trend-icon">
                  <path d="M2 4.5 6.1 8.6l2.6-2.6L14 11.3"></path>
                  <path d="M10.5 11.3H14V7.8"></path>
                </svg>
              }
              <span>{{ monthlyComparisonLabel() }}</span>
            </small>
          </article>
          <article class="month-strip-item">
            <span>{{ messages.hero.monthIncome }}</span>
            <strong>{{ money(currentMonthIncome()) }}</strong>
            <small>{{ messages.metrics.monthIncomeHint }}</small>
          </article>
          <article class="month-strip-item">
            <span>{{ messages.hero.monthExpenses }}</span>
            <strong>{{ money(currentMonthExpenses()) }}</strong>
            <small>{{ messages.metrics.monthExpensesHint }}</small>
          </article>
        </div>
      </section>

      <section class="dashboard-workspace">
        <div class="dashboard-main-column">
          <article class="panel panel-surface trend-panel">
            <div class="panel-header">
              <div>
                <h2>{{ messages.cashflow.title }}</h2>
                <p>{{ messages.cashflow.subtitle }}</p>
              </div>
              <div class="chart-legend">
                <span><i class="legend-swatch income"></i>{{ messages.legend.income }}</span>
                <span><i class="legend-swatch expense"></i>{{ messages.legend.expense }}</span>
              </div>
            </div>
            @if (hasChartData()) {
              <div class="chart-card">
                @if (hoveredTimelineIndex() !== null) {
                  <div
                    class="chart-hover-tooltip"
                    [style.left.%]="selectedTooltipLeftPercent(640, 72, 20)"
                    [class.align-left]="selectedMonthIndex() <= 1"
                    [class.align-right]="selectedMonthIndex() >= 10"
                  >
                    <strong>{{ selectedMonthLabel() }}</strong>
                    <span>{{ messages.legend.income }}: {{ money(selectedMonthIncome()) }}</span>
                    <span>{{ messages.legend.expense }}: {{ money(selectedMonthExpenses()) }}</span>
                    <span>{{ messages.hero.highlight }}: {{ signedMoney(selectedMonthNet()) }}</span>
                  </div>
                }
                <svg
                  viewBox="0 0 640 260"
                  class="chart-svg"
                  role="img"
                  [attr.aria-label]="messages.cashflow.aria"
                  (mousemove)="onTimelineHover($event, 640, 72, 20, rollingMonthLabels().length)"
                  (mouseleave)="clearTimelineHover()"
                >
                  <defs>
                    <linearGradient id="cashflow-income-fill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stop-color="#5f9d8c" stop-opacity="0.34" />
                      <stop offset="100%" stop-color="#5f9d8c" stop-opacity="0" />
                    </linearGradient>
                    <linearGradient id="cashflow-expense-fill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stop-color="#d08b62" stop-opacity="0.3" />
                      <stop offset="100%" stop-color="#d08b62" stop-opacity="0" />
                    </linearGradient>
                  </defs>
                  <rect x="72" y="20" width="548" height="206" rx="18" class="chart-plot-bg" />
                  @for (tick of chartTicks(); track tick) {
                    <text x="64" [attr.y]="cashflowY(tick) + 3" text-anchor="end" class="chart-value-label">{{ axisMoney(tick) }}</text>
                    <line
                      x1="72"
                      x2="620"
                      [attr.y1]="cashflowY(tick)"
                      [attr.y2]="cashflowY(tick)"
                      class="chart-grid-line"
                    />
                  }
                  <rect
                    [attr.x]="currentCashflowX() - 24"
                    y="20"
                    width="48"
                    height="206"
                    rx="24"
                    class="chart-active-window"
                  />
                  <line
                    [attr.x1]="currentCashflowX()"
                    [attr.x2]="currentCashflowX()"
                    y1="20"
                    y2="226"
                    class="chart-focus-line"
                  />
                  <path [attr.d]="cashflowAreaPath(incomeMonthlyTotals())" class="chart-area income-area" fill="url(#cashflow-income-fill)"></path>
                  <path [attr.d]="cashflowAreaPath(expenseMonthlyTotalsAbsolute())" class="chart-area expense-area" fill="url(#cashflow-expense-fill)"></path>
                  <path [attr.d]="cashflowLinePath(expenseMonthlyTotalsAbsolute())" class="chart-line expense-line"></path>
                  <path [attr.d]="cashflowLinePath(incomeMonthlyTotals())" class="chart-line income-line"></path>
                  @for (value of expenseMonthlyTotalsAbsolute(); track $index; let monthIndex = $index) {
                    <circle [attr.cx]="cashflowX(monthIndex)" [attr.cy]="cashflowY(value)" r="3.5" class="chart-point expense-point"></circle>
                  }
                  @for (value of incomeMonthlyTotals(); track $index; let monthIndex = $index) {
                    <circle [attr.cx]="cashflowX(monthIndex)" [attr.cy]="cashflowY(value)" r="3.5" class="chart-point income-point"></circle>
                  }
                  <circle [attr.cx]="currentCashflowX()" [attr.cy]="cashflowY(selectedMonthIncome())" r="5" class="chart-focus-dot income-dot"></circle>
                  <circle [attr.cx]="currentCashflowX()" [attr.cy]="cashflowY(selectedMonthExpenses())" r="5" class="chart-focus-dot expense-dot"></circle>
                  @for (month of rollingMonthLabels(); track month; let monthIndex = $index) {
                    <text [attr.x]="cashflowX(monthIndex)" y="254" text-anchor="middle" class="chart-label" [class.chart-label-active]="monthIndex === selectedMonthIndex()">{{ month }}</text>
                  }
                </svg>
              </div>
            } @else {
              <p class="state-message">{{ messages.cashflow.empty }}</p>
            }
          </article>

          <article class="panel panel-surface transactions-panel">
            <div class="panel-header">
              <div>
                <h2>{{ messages.recentTransactions.title }}</h2>
                <p>{{ messages.recentTransactions.subtitle }}</p>
              </div>
            </div>
            @if (transactions().length === 0) {
              <p class="state-message panel-state-message">{{ messages.recentTransactions.empty }}</p>
            } @else {
              <div class="transaction-list">
                @for (tx of transactions(); track tx.id) {
                  <div class="transaction-row">
                    <div class="transaction-copy">
                      <strong>{{ tx.description }}</strong>
                      <span>{{ transactionMeta(tx) }}</span>
                    </div>
                    <strong [class.negative-value]="tx.amount < 0" [class.positive-value]="tx.amount > 0">{{ signedMoney(tx.amount) }}</strong>
                  </div>
                }
              </div>
            }
          </article>

          <article class="panel panel-surface category-trends-panel">
            <div class="panel-header category-trends-header">
              <div>
                <h2>{{ messages.categoryTrends.title }}</h2>
                <p>{{ messages.categoryTrends.subtitle }}</p>
              </div>
              <div class="chart-legend">
                @for (series of expenseTrendSeries(); track series.code) {
                  <span><i class="legend-swatch" [style.background]="series.color"></i>{{ series.name }}</span>
                }
              </div>
            </div>
            @if (hasExpenseTrendData()) {
              <div class="chart-card">
                @if (hoveredTimelineIndex() !== null) {
                  <div
                    class="chart-hover-tooltip chart-hover-tooltip-wide"
                    [style.left.%]="selectedTooltipLeftPercent(640, 72, 20)"
                    [class.align-left]="selectedMonthIndex() <= 1"
                    [class.align-right]="selectedMonthIndex() >= 10"
                  >
                    <strong>{{ selectedMonthLabel() }}</strong>
                    <div class="chart-hover-list">
                      @for (item of selectedExpenseTrendItems(); track item.code) {
                        <div class="chart-hover-list-row">
                          <span class="chart-hover-list-label">
                            <i class="legend-swatch" [style.background]="item.color"></i>
                            {{ item.name }}
                          </span>
                          <b>{{ moneyAbsolute(item.value) }}</b>
                        </div>
                      }
                    </div>
                  </div>
                }
                <svg
                  viewBox="0 0 640 260"
                  class="chart-svg"
                  role="img"
                  [attr.aria-label]="messages.categoryTrends.aria"
                  (mousemove)="onTimelineHover($event, 640, 72, 20, rollingMonthLabels().length)"
                  (mouseleave)="clearTimelineHover()"
                >
                  <rect x="72" y="20" width="548" height="206" rx="18" class="chart-plot-bg" />
                  @for (tick of expenseTrendTicks(); track tick) {
                    <text x="64" [attr.y]="expenseTrendY(tick) + 3" text-anchor="end" class="chart-value-label">{{ axisMoney(tick) }}</text>
                    <line
                      x1="72"
                      x2="620"
                      [attr.y1]="expenseTrendY(tick)"
                      [attr.y2]="expenseTrendY(tick)"
                      class="chart-grid-line"
                    />
                  }
                  <rect
                    [attr.x]="currentCashflowX() - 24"
                    y="20"
                    width="48"
                    height="206"
                    rx="24"
                    class="chart-active-window subtle"
                  />
                  <line
                    [attr.x1]="currentCashflowX()"
                    [attr.x2]="currentCashflowX()"
                    y1="20"
                    y2="226"
                    class="chart-focus-line subtle"
                  />
                  @for (series of expenseTrendSeries(); track series.code) {
                    <path [attr.d]="expenseTrendLinePath(series.monthly)" class="chart-line category-trend-line" [attr.stroke]="series.color">
                      <title>{{ series.name }}</title>
                    </path>
                    <circle
                      [attr.cx]="currentCashflowX()"
                      [attr.cy]="expenseTrendY(series.monthly[selectedMonthIndex()])"
                      r="3.5"
                      class="chart-point category-trend-point"
                      [attr.fill]="series.color"
                    ></circle>
                  }
                  @for (month of rollingMonthLabels(); track month; let monthIndex = $index) {
                    <text [attr.x]="cashflowX(monthIndex)" y="254" text-anchor="middle" class="chart-label" [class.chart-label-active]="monthIndex === selectedMonthIndex()">{{ month }}</text>
                  }
                </svg>
              </div>
            } @else {
              <p class="state-message">{{ messages.categoryTrends.empty }}</p>
            }
          </article>

          <article class="panel panel-surface category-benchmark-panel">
            <div class="panel-header category-benchmark-header">
              <div>
                <h2>{{ messages.categoryBenchmark.title }}</h2>
                <p>{{ messages.categoryBenchmark.subtitle }}</p>
              </div>
            </div>
            @if (!hasCategoryBenchmarkHistory()) {
              <p class="state-message panel-state-message">{{ messages.categoryBenchmark.historyUnavailable }}</p>
            } @else if (categoryBenchmarkRows().length === 0) {
              <p class="state-message panel-state-message">{{ messages.categoryBenchmark.empty }}</p>
            } @else {
              <div class="category-benchmark-card">
                @for (row of categoryBenchmarkRows(); track row.code) {
                  <div class="benchmark-row">
                    <div class="benchmark-copy">
                      <div class="benchmark-copy-header">
                        <strong>{{ row.name }}</strong>
                      </div>
                      <div class="benchmark-values">
                        <span>{{ messages.categoryBenchmark.currentLabel }}: <b>{{ moneyAbsolute(row.current) }}</b></span>
                        <span>{{ messages.categoryBenchmark.averageLabel }}: <b>{{ moneyAbsolute(row.baseline) }}</b></span>
                      </div>
                    </div>
                    <span class="benchmark-status" [class.above]="row.status === 'above'" [class.near]="row.status === 'near'" [class.below]="row.status === 'below'">
                      {{ categoryBenchmarkStatusLabel(row.status) }}
                    </span>
                    <div class="benchmark-delta">
                      <strong [class.negative-value]="row.delta > 0" [class.positive-value]="row.delta < 0">
                        {{ signedMoneyAbsolute(row.delta) }}
                      </strong>
                    </div>
                  </div>
                }
              </div>
            }
          </article>
        </div>

        <aside class="dashboard-side-column">
          <article class="panel panel-surface insights-panel">
            <div class="panel-header">
              <div>
                <h2>{{ messages.insights.title }}</h2>
                <p>{{ messages.insights.subtitle }}</p>
              </div>
            </div>
            <div class="insights-list">
              <div class="insights-summary">
                <div class="insight-metric">
                  <span>{{ messages.insights.savingsRate }}</span>
                  <strong [class.insight-metric-fallback]="!hasMonthlySavingsRate()">{{ monthlySavingsRateLabel() }}</strong>
                </div>
                <div class="insight-metric">
                  <span>{{ messages.insights.changeVsPrevious }}</span>
                  <strong [class.negative-value]="currentMonthNet() - previousMonthNet() < 0">{{ signedMoney(currentMonthNet() - previousMonthNet()) }}</strong>
                </div>
              </div>
              <div class="insight-row compact">
                <span>{{ messages.insights.topExpense }}</span>
                @if (topExpenseTransactions().length > 0) {
                  <div class="insight-expense-list">
                    @for (expense of topExpenseTransactions(); track expense.id; let itemIndex = $index) {
                      <div class="insight-expense-item">
                        <b>{{ itemIndex + 1 }}.</b>
                        <strong [title]="expense.description">{{ expense.description }}</strong>
                        <small>{{ signedMoney(expense.amount) }} • {{ expenseShareOfMonthLabel(expense.amount) }}</small>
                      </div>
                    }
                  </div>
                } @else {
                  <div class="insight-detail">
                    <strong>{{ messages.insights.topExpenseEmpty }}</strong>
                  </div>
                }
              </div>
            </div>
          </article>

          <article class="panel panel-surface breakdown-panel">
            <div class="panel-header">
              <div>
                <h2>{{ messages.expenseBreakdown.title }}</h2>
                <p>{{ messages.expenseBreakdown.subtitle }}</p>
              </div>
            </div>
            @if (expenseBreakdown().length === 0) {
              <p class="state-message">{{ messages.expenseBreakdown.empty }}</p>
            } @else {
              <div class="donut-layout">
                <div class="donut-wrap" [style.--donut-size]="'180px'">
                  <svg viewBox="0 0 120 120" class="donut-chart" role="img" [attr.aria-label]="messages.expenseBreakdown.aria">
                    <circle cx="60" cy="60" r="42" class="donut-track"></circle>
                    @for (slice of expenseBreakdown(); track slice.code) {
                      <circle
                        cx="60"
                        cy="60"
                        r="42"
                        class="donut-slice"
                        [attr.stroke]="slice.color"
                        [attr.stroke-dasharray]="slice.dashArray"
                        [attr.stroke-dashoffset]="slice.dashOffset"
                      ></circle>
                    }
                  </svg>
                  <div class="donut-center">
                    <span>{{ messages.expenseBreakdown.centerLabel }}</span>
                    <strong>{{ money(ytdExpenses()) }}</strong>
                  </div>
                </div>
                <div class="breakdown-list">
                  @for (item of expenseBreakdown(); track item.code) {
                    <div class="breakdown-row">
                      <div class="breakdown-meta">
                        <i class="breakdown-dot" [style.background]="item.color"></i>
                        <div>
                          <strong>{{ item.name }}</strong>
                          <span>{{ item.shareLabel }}</span>
                        </div>
                      </div>
                      <b>{{ money(item.amount) }}</b>
                    </div>
                  }
                </div>
              </div>
            }
          </article>

          <article class="panel panel-surface spotlight-panel">
            <div class="panel-header">
              <div>
                <h2>{{ messages.accumulated.title }}</h2>
                <p>{{ messages.accumulated.subtitle }}</p>
              </div>
            </div>
            @if (hasChartData()) {
              <div class="spotlight-total">
                <span>{{ messages.accumulated.total }}</span>
                <strong [class.negative-value]="ytdNet() < 0">{{ signedMoney(ytdNet()) }}</strong>
              </div>
              <svg viewBox="0 0 360 170" class="mini-chart" role="img" [attr.aria-label]="messages.accumulated.aria">
                <defs>
                  <linearGradient id="accumulated-fill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#5f9d8c" stop-opacity="0.28" />
                    <stop offset="100%" stop-color="#5f9d8c" stop-opacity="0.02" />
                  </linearGradient>
                </defs>
                <rect x="16" y="16" width="326" height="126" rx="16" class="chart-plot-bg mini-plot-bg" />
                <path [attr.d]="accumulatedAreaPath()" class="mini-area"></path>
                <path [attr.d]="accumulatedLinePath()" class="mini-line"></path>
                <circle [attr.cx]="miniChartX(selectedMonthIndex())" [attr.cy]="accumulatedY(selectedAccumulatedTotal())" r="4.5" class="chart-focus-dot mini-dot"></circle>
                @for (month of rollingMonthLabels(); track month; let monthIndex = $index) {
                  @if (shouldShowMiniLabel(monthIndex)) {
                    <text [attr.x]="miniLabelX(monthIndex)" y="166" text-anchor="middle" class="chart-label mini-label">{{ month }}</text>
                  }
                }
              </svg>
            } @else {
              <p class="state-message">{{ messages.accumulated.empty }}</p>
            }
          </article>

          <article class="panel panel-surface balances-panel">
            <div class="panel-header">
              <div>
                <h2>{{ messages.balances.title }}</h2>
                <p>{{ balancesSummaryLabel() }}</p>
              </div>
            </div>
            @if (accountSummaries().length === 0) {
              <p class="state-message">{{ messages.balances.empty }}</p>
            } @else {
              <div class="balance-bars">
                @for (account of accountSummaries(); track account.code) {
                  <div class="balance-row">
                    <div class="balance-row-header">
                      <div>
                        <strong>{{ account.name }}</strong>
                        <span>{{ account.typeLabel }}</span>
                      </div>
                      <b [class.negative-value]="account.balance < 0">{{ money(account.balance) }}</b>
                    </div>
                    <div class="balance-track">
                      <div class="balance-fill" [style.width.%]="account.width" [style.background]="account.color"></div>
                    </div>
                  </div>
                }
              </div>
            }
          </article>
        </aside>
      </section>
    }
  `,
})
export class DashboardComponent implements OnInit {
  readonly messages = uiMessages.dashboard;
  readonly months = uiMessages.labels.monthsShort;
  readonly minYear = 2000;
  readonly loading = signal(true);
  readonly error = signal('');
  readonly currentYear = signal(new Date().getFullYear());
  readonly currentMonth = signal(new Date().getMonth() + 1);
  readonly pickerYear = signal(new Date().getFullYear());
  readonly periodPickerOpen = signal(false);
  readonly hoveredTimelineIndex = signal<number | null>(null);
  readonly balances = signal<AccountBalance[]>([]);
  readonly transactions = signal<Transaction[]>([]);
  readonly topExpenses = signal<Transaction[]>([]);
  readonly yearly = signal<CategoryYearlyBalance[]>([]);
  readonly dashboardMetricSkeletonItems = [0, 1, 2];
  readonly dashboardInsightSkeletonItems = [0, 1];
  readonly dashboardListSkeletonItems = [0, 1, 2, 3];
  readonly dashboardBalanceSkeletonItems = [0, 1, 2, 3, 4];
  readonly incomeRows = computed(() => this.rowsForType('INCOME'));
  readonly expenseRows = computed(() => this.rowsForType('EXPENSE'));
  readonly accountSummaries = computed(() => this.buildAccountSummaries());
  readonly expenseBreakdown = computed(() => this.buildExpenseBreakdown());
  readonly monthlyExpenseBreakdown = computed(() => this.buildMonthlyExpenseBreakdown());
  readonly expenseTrendSeries = computed(() => this.buildExpenseTrendSeries());
  readonly categoryBenchmarkRows = computed(() => this.buildCategoryBenchmarkRows());

  constructor(
    private readonly elementRef: ElementRef<HTMLElement>,
    private readonly referenceData: ReferenceDataService,
    private readonly reports: ReportsService,
    private readonly moneyVisibility: MoneyVisibilityService,
    private readonly toast: ToastService,
  ) {}

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent): void {
    if (!this.periodPickerOpen()) {
      return;
    }
    if (!this.elementRef.nativeElement.contains(event.target as Node)) {
      this.periodPickerOpen.set(false);
    }
  }

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.loading.set(true);

    forkJoin({
      referenceData: this.referenceData.load(),
      dashboard: this.reports.dashboard(this.currentYear(), this.currentMonth()),
    }).subscribe({
      next: ({ dashboard }) => {
        this.currentYear.set(dashboard.year);
        this.currentMonth.set(dashboard.month);
        this.pickerYear.set(dashboard.year);
        this.balances.set(dashboard.balances);
        this.yearly.set(dashboard.yearly);
        this.transactions.set(dashboard.recent_transactions);
        this.topExpenses.set(dashboard.top_expenses);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }

  totalBalance(): number {
    return this.balances().reduce((total, item) => total + item.Balance, 0);
  }

  currentMonthIncome(): number {
    return this.incomeMonthlyTotals()[this.activeTimelineIndex()] ?? 0;
  }

  currentMonthExpenses(): number {
    return this.expenseMonthlyTotalsAbsolute()[this.activeTimelineIndex()] ?? 0;
  }

  currentMonthNet(): number {
    return this.netMonthlyTotals()[this.activeTimelineIndex()] ?? 0;
  }

  selectedMonthIncome(): number {
    return this.incomeMonthlyTotals()[this.selectedMonthIndex()] ?? 0;
  }

  selectedMonthExpenses(): number {
    return this.expenseMonthlyTotalsAbsolute()[this.selectedMonthIndex()] ?? 0;
  }

  selectedMonthNet(): number {
    return this.netMonthlyTotals()[this.selectedMonthIndex()] ?? 0;
  }

  selectedAccumulatedTotal(): number {
    return this.accumulatedMonthlyTotals()[this.selectedMonthIndex()] ?? 0;
  }

  previousMonthNet(): number {
    return this.netMonthlyTotals()[Math.max(0, this.activeTimelineIndex() - 1)] ?? 0;
  }

  periodLabel(): string {
    return `${this.months[this.currentMonthIndex()]} ${this.currentYear()}`;
  }

  selectedMonthLabel(): string {
    return this.rollingMonthLabels()[this.selectedMonthIndex()] ?? this.periodLabel();
  }

  ytdIncome(): number {
    return this.incomeMonthlyTotals().reduce((total, value) => total + value, 0);
  }

  ytdExpenses(): number {
    return this.expenseMonthlyTotalsAbsolute().reduce((total, value) => total + value, 0);
  }

  ytdNet(): number {
    return this.netMonthlyTotals().reduce((total, value) => total + value, 0);
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrency(value);
  }

  moneyAbsolute(value: number): string {
    return this.moneyVisibility.formatCurrencyAbsolute(value);
  }

  assetAccountsLabel(): string {
    return 'Recursos';
  }

  liabilityAccountsLabel(): string {
    return 'Contas a pagar';
  }

  signedMoney(value: number): string {
    return this.moneyVisibility.formatSignedCurrency(value);
  }

  signedMoneyAbsolute(value: number): string {
    const absolute = this.moneyVisibility.formatCurrencyAbsolute(Math.abs(value));
    return value > 0 ? `+ ${absolute}` : value < 0 ? `- ${absolute}` : absolute;
  }

  heroSummary(): string {
    if (this.ytdNet() > 0) {
      return this.messages.hero.positiveSummary;
    }
    if (this.ytdNet() < 0) {
      return this.messages.hero.negativeSummary;
    }
    return this.messages.hero.neutralSummary;
  }

  savingsRateLabel(): string {
    const income = this.ytdIncome();
    if (income <= 0) {
      return this.messages.metrics.noSavingsRate;
    }
    if (this.ytdNet() <= 0) {
      return this.messages.metrics.noSavingsLeftYet;
    }
    const rate = Math.round((this.ytdNet() / income) * 100);
    return this.messages.metrics.savingsRate.replace('{rate}', String(rate));
  }

  monthlySavingsRateLabel(): string {
    const income = this.currentMonthIncome();
    if (income <= 0) {
      return this.messages.metrics.noSavingsRate;
    }
    if (this.currentMonthNet() <= 0) {
      return this.messages.metrics.noSavingsLeftYet;
    }
    const rate = Math.round((this.currentMonthNet() / income) * 100);
    return this.messages.metrics.savingsRate.replace('{rate}', String(rate));
  }

  hasMonthlySavingsRate(): boolean {
    return this.currentMonthIncome() > 0 && this.currentMonthNet() > 0;
  }

  monthlyComparisonLabel(): string {
    const current = this.currentMonthNet();
    const previous = this.previousMonthNet();
    const delta = current - previous;
    if (delta === 0) {
      return this.messages.hero.sameAsPrevious;
    }
    const direction = delta > 0 ? this.messages.hero.abovePrevious : this.messages.hero.belowPrevious;
    return `${this.moneyVisibility.formatCurrencyAbsolute(delta)} ${direction}`;
  }

  monthlyComparisonDelta(): number {
    return this.currentMonthNet() - this.previousMonthNet();
  }

  balancesSummaryLabel(): string {
    const assets = this.accountSummaries().filter((account) => account.type === 'ASSET').length;
    const liabilities = this.accountSummaries().filter((account) => account.type === 'LIABILITY').length;
    return this.messages.balances.summary
      .replace('{assets}', String(assets))
      .replace('{liabilities}', String(liabilities));
  }

  transactionMeta(transaction: Transaction): string {
    const category = this.referenceData.categoryName(transaction.category_code);
    const account = this.referenceData.accountName(transaction.account_code);
    return `${toBrazilianDate(transaction.date)} • ${category} • ${account}`;
  }

  axisMoney(value: number): string {
    return this.moneyVisibility.formatCompactCurrencyAbsolute(value);
  }

  topExpenseItem(): ExpenseBreakdownItem | null {
    return this.monthlyExpenseBreakdown()[0] ?? null;
  }

  topExpenseTransactions(): Transaction[] {
    return this.topExpenses();
  }

  topExpenseShareLabel(): string {
    const expense = this.topExpenseTransactions()[0];
    if (!expense) {
      return '0%';
    }
    const total = this.currentMonthExpenses();
    if (total <= 0) {
      return '0%';
    }
    return `${Math.round((Math.abs(expense.amount) / total) * 100)}%`;
  }

  expenseShareOfMonthLabel(amount: number): string {
    const total = this.currentMonthExpenses();
    if (total <= 0) {
      return '0% do mes';
    }
    return `${Math.round((Math.abs(amount) / total) * 100)}% do mes`;
  }

  topExpenseSummaryLabel(): string {
    const expense = this.topExpenseTransactions()[0];
    if (!expense) {
      return this.periodLabel();
    }
    return `${expense.description} representa ${this.expenseShareOfMonthLabel(expense.amount)}`;
  }

  hasExpenseTrendData(): boolean {
    return this.expenseTrendSeries().some((series) => series.monthly.some((value) => value > 0));
  }

  selectedExpenseTrendItems(): Array<{ code: string; name: string; color: string; value: number }> {
    return this.expenseTrendSeries()
      .map((series) => ({
        code: series.code,
        name: series.name,
        color: series.color,
        value: series.monthly[this.selectedMonthIndex()] ?? 0,
      }))
      .filter((item) => item.value > 0)
      .sort((left, right) => right.value - left.value);
  }

  hasCategoryBenchmarkHistory(): boolean {
    const startIndex = Math.max(0, this.activeTimelineIndex() - 6);
    return this.expenseRows().some((row) =>
      row.monthly
        .slice(startIndex, this.activeTimelineIndex())
        .some((value) => Math.abs(value) > 0),
    );
  }

  categoryBenchmarkStatusLabel(status: CategoryBenchmarkStatus): string {
    switch (status) {
      case 'above':
        return this.messages.categoryBenchmark.statusAbove;
      case 'below':
        return this.messages.categoryBenchmark.statusBelow;
      default:
        return this.messages.categoryBenchmark.statusNear;
    }
  }

  currentCashflowX(): number {
    return this.cashflowX(this.selectedMonthIndex());
  }

  selectedMonthIndex(): number {
    return this.hoveredTimelineIndex() ?? this.activeTimelineIndex();
  }

  selectedTooltipLeftPercent(width: number, left: number, right: number): number {
    return (this.pointX(this.selectedMonthIndex(), width, left, right, this.rollingMonthLabels().length) / width) * 100;
  }

  totalAssetsBalance(): number {
    const accountMap = new Map<string, Account>(this.referenceData.accounts().map((account) => [account.Code, account]));
    return this.balances().reduce((total, item) => {
      const account = accountMap.get(item.AccountCode);
      return account?.Type === 'ASSET' ? total + item.Balance : total;
    }, 0);
  }

  totalLiabilitiesBalance(): number {
    const accountMap = new Map<string, Account>(this.referenceData.accounts().map((account) => [account.Code, account]));
    return this.balances().reduce((total, item) => {
      const account = accountMap.get(item.AccountCode);
      return account?.Type === 'LIABILITY' ? total + Math.abs(item.Balance) : total;
    }, 0);
  }

  onTimelineHover(event: MouseEvent, width: number, left: number, right: number, points: number): void {
    const svg = event.currentTarget as SVGElement | null;
    if (!svg || points <= 1) {
      return;
    }

    const rect = svg.getBoundingClientRect();
    const pointerX = ((event.clientX - rect.left) / rect.width) * width;
    const usableWidth = width - left - right;
    const clampedX = Math.min(Math.max(pointerX, left), width - right);
    const ratio = usableWidth === 0 ? 0 : (clampedX - left) / usableWidth;
    const index = Math.round(ratio * (points - 1));
    this.hoveredTimelineIndex.set(Math.max(0, Math.min(points - 1, index)));
  }

  clearTimelineHover(): void {
    this.hoveredTimelineIndex.set(null);
  }

  shouldShowMiniLabel(index: number): boolean {
    return index === 0 || index === 4 || index === 8 || index === 11;
  }

  rollingMonthLabels(): string[] {
    const labels: string[] = [];
    const selected = new Date(this.currentYear(), this.currentMonth() - 1, 1);
    for (let offset = 11; offset >= 0; offset -= 1) {
      const value = new Date(selected.getFullYear(), selected.getMonth() - offset, 1);
      labels.push(`${this.months[value.getMonth()]}/${String(value.getFullYear()).slice(-2)}`);
    }
    return labels;
  }

  yearOptions(): number[] {
    const now = new Date();
    return Array.from({ length: now.getFullYear() - this.minYear + 1 }, (_, index) => now.getFullYear() - index);
  }

  togglePeriodPicker(): void {
    this.pickerYear.set(this.currentYear());
    this.periodPickerOpen.update((value) => !value);
  }

  onPickerYearChange(value: string): void {
    const year = Number(value);
    if (Number.isInteger(year)) {
      this.pickerYear.set(year);
    }
  }

  isFutureMonth(year: number, month: number): boolean {
    const now = new Date();
    return year > now.getFullYear() || (year === now.getFullYear() && month > now.getMonth() + 1);
  }

  pickMonth(year: number, month: number): void {
    if (this.isFutureMonth(year, month)) {
      return;
    }
    this.currentYear.set(year);
    this.currentMonth.set(month);
    this.pickerYear.set(year);
    this.periodPickerOpen.set(false);
    this.load();
  }

  goToPreviousMonth(): void {
    const month = this.currentMonth();
    const year = this.currentYear();

    if (month === 1) {
      if (year <= this.minYear) {
        return;
      }
      this.currentYear.set(year - 1);
      this.currentMonth.set(12);
    } else {
      this.currentMonth.set(month - 1);
    }

    this.load();
  }

  goToNextMonth(): void {
    if (!this.canGoToNextMonth()) {
      return;
    }

    const month = this.currentMonth();
    const year = this.currentYear();

    if (month === 12) {
      this.currentYear.set(year + 1);
      this.currentMonth.set(1);
    } else {
      this.currentMonth.set(month + 1);
    }

    this.load();
  }

  canGoToNextMonth(): boolean {
    const now = new Date();
    const currentYear = now.getFullYear();
    const currentMonth = now.getMonth() + 1;

    return this.currentYear() < currentYear || (this.currentYear() === currentYear && this.currentMonth() < currentMonth);
  }

  hasChartData(): boolean {
    return this.incomeMonthlyTotals().some((value) => value > 0) || this.expenseMonthlyTotalsAbsolute().some((value) => value > 0);
  }

  chartTicks(): number[] {
    const max = this.cashflowMax();
    return [0, max * 0.33, max * 0.66, max];
  }

  expenseTrendTicks(): number[] {
    const max = this.expenseTrendMax();
    return [0, max * 0.33, max * 0.66, max];
  }

  cashflowLinePath(values: number[]): string {
    return this.linePath(values, 640, 260, 18, 20, 34, this.cashflowMax(), 0, 72);
  }

  cashflowAreaPath(values: number[]): string {
    return this.areaPath(values, 640, 260, 18, 20, 34, this.cashflowMax(), 0, 72);
  }

  cashflowX(index: number): number {
    return this.pointX(index, 640, 72, 20, this.months.length);
  }

  cashflowY(value: number): number {
    return this.pointY(value, 260, 18, 34, this.cashflowMax(), 0);
  }

  expenseTrendLinePath(values: number[]): string {
    return this.linePath(values, 640, 260, 18, 20, 34, this.expenseTrendMax(), 0, 72);
  }

  expenseTrendY(value: number): number {
    return this.pointY(value, 260, 18, 34, this.expenseTrendMax(), 0);
  }

  accumulatedY(value: number): number {
    const totals = this.accumulatedMonthlyTotals();
    const { min, max } = this.minMax(totals);
    return this.pointY(value, 170, 16, 18, max, min);
  }

  accumulatedLinePath(): string {
    const totals = this.accumulatedMonthlyTotals();
    const { min, max } = this.minMax(totals);
    return this.linePath(totals, 360, 170, 16, 18, 18, max, min);
  }

  accumulatedAreaPath(): string {
    const totals = this.accumulatedMonthlyTotals();
    const { min, max } = this.minMax(totals);
    return this.areaPath(totals, 360, 170, 16, 18, 18, max, min);
  }

  miniChartX(index: number): number {
    return this.pointX(index, 360, 16, 18, this.months.length);
  }

  miniLabelX(index: number): number {
    const x = this.miniChartX(index);
    if (index === 0) {
      return x + 10;
    }
    return x;
  }

  incomeMonthlyTotals(): number[] {
    return this.sumMonthly(this.incomeRows().map((row) => row.monthly));
  }

  expenseMonthlyTotalsRaw(): number[] {
    return this.sumMonthly(this.expenseRows().map((row) => row.monthly));
  }

  expenseMonthlyTotalsAbsolute(): number[] {
    return this.expenseMonthlyTotalsRaw().map((value) => Math.abs(value));
  }

  netMonthlyTotals(): number[] {
    const income = this.incomeMonthlyTotals();
    const expense = this.expenseMonthlyTotalsRaw();
    return income.map((value, index) => value + (expense[index] ?? 0));
  }

  accumulatedMonthlyTotals(): number[] {
    let total = 0;
    return this.netMonthlyTotals().map((value) => {
      total += value;
      return total;
    });
  }

  private rowsForType(type: CategoryType): ReportRow[] {
    return this.yearly()
      .map((category) => this.categoryToReportRow(category, type))
      .filter((row): row is ReportRow => row !== null);
  }

  private categoryToReportRow(category: CategoryYearlyBalance, type: CategoryType): ReportRow | null {
    const children = (category.subcategories ?? [])
      .filter((child) => this.categoryType(child.code) === type)
      .map((child) => ({
        code: child.code,
        name: this.referenceData.categoryName(child.code),
        monthly: child.monthly_data,
        children: [],
      }));

    if (this.categoryType(category.code) !== type) {
      return null;
    }

    return {
      code: category.code,
      name: this.referenceData.categoryName(category.code),
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

  private currentMonthIndex(): number {
    return this.currentMonth() - 1;
  }

  private activeTimelineIndex(): number {
    return 11;
  }

  private buildAccountSummaries(): AccountSummary[] {
    const accountMap = new Map<string, Account>(this.referenceData.accounts().map((account) => [account.Code, account]));
    const visibleBalances = this.balances().filter((item) => item.Balance !== 0);
    const maxBalance = Math.max(...visibleBalances.map((item) => Math.abs(item.Balance)), 1);

    return visibleBalances
      .map((item) => {
        const account = accountMap.get(item.AccountCode);
        const type = account?.Type ?? 'ASSET';
        return {
          code: item.AccountCode,
          name: account?.Name ?? item.AccountCode,
          type,
          typeLabel: type === 'LIABILITY' ? this.messages.balances.liability : this.messages.balances.asset,
          balance: item.Balance,
          width: (Math.abs(item.Balance) / maxBalance) * 100,
          color: type === 'LIABILITY' ? '#d7644a' : '#1f8f68',
        };
      })
      .sort((left, right) => Math.abs(right.balance) - Math.abs(left.balance));
  }

  private buildExpenseBreakdown(): ExpenseBreakdownItem[] {
    const source = this.expenseRows()
      .flatMap((row) => row.children.length > 0 ? row.children : [row])
      .map((row) => ({
        code: row.code,
        name: row.name,
        amount: Math.abs(row.monthly.reduce((total, value) => total + value, 0)),
      }))
      .filter((row) => row.amount > 0)
      .sort((left, right) => right.amount - left.amount);

    if (source.length === 0) {
      return [];
    }

    const topItems = source.slice(0, 10);
    const remainder = source.slice(10).reduce((total, item) => total + item.amount, 0);
    const items = remainder > 0 ? [...topItems, { code: 'other', name: this.messages.expenseBreakdown.other, amount: remainder }] : topItems;
    const total = items.reduce((sum, item) => sum + item.amount, 0);
    const circumference = 2 * Math.PI * 42;
    let consumed = 0;

    return items.map((item, index) => {
      const share = item.amount / total;
      const length = share * circumference;
      const result = {
        ...item,
        color: DONUT_COLORS[index % DONUT_COLORS.length],
        shareLabel: `${Math.round(share * 100)}%`,
        dashArray: `${length} ${circumference - length}`,
        dashOffset: `${-consumed}`,
      };
      consumed += length;
      return result;
    });
  }

  private buildMonthlyExpenseBreakdown(): ExpenseBreakdownItem[] {
    const source = this.expenseRows()
      .flatMap((row) => row.children.length > 0 ? row.children : [row])
      .map((row) => ({
        code: row.code,
        name: row.name,
        amount: Math.abs(row.monthly[this.activeTimelineIndex()] ?? 0),
      }))
      .filter((row) => row.amount > 0)
      .sort((left, right) => right.amount - left.amount);

    if (source.length === 0) {
      return [];
    }

    const total = source.reduce((sum, item) => sum + item.amount, 0);
    return source.slice(0, 5).map((item, index) => ({
      ...item,
      color: DONUT_COLORS[index % DONUT_COLORS.length],
      shareLabel: `${Math.round((item.amount / total) * 100)}%`,
      dashArray: '',
      dashOffset: '',
    }));
  }

  private buildExpenseTrendSeries(): TrendSeries[] {
    return this.expenseRows()
      .map((row) => ({
        code: row.code,
        name: row.name,
        monthly: row.monthly.map((value) => Math.abs(value)),
      }))
      .map((series) => ({
        ...series,
        total: series.monthly.reduce((sum, value) => sum + value, 0),
      }))
      .filter((series) => series.total > 0)
      .sort((left, right) => right.total - left.total)
      .slice(0, 10)
      .map((series, index) => ({
        code: series.code,
        name: series.name,
        monthly: series.monthly,
        color: TREND_COLORS[index % TREND_COLORS.length],
      }));
  }

  private buildCategoryBenchmarkRows(): CategoryBenchmarkRow[] {
    if (!this.hasCategoryBenchmarkHistory()) {
      return [];
    }

    return this.expenseRows()
      .map((row) => {
        const current = Math.abs(row.monthly[this.activeTimelineIndex()] ?? 0);
        const previousMonths = row.monthly
          .slice(Math.max(0, this.activeTimelineIndex() - 6), this.activeTimelineIndex())
          .map((value) => Math.abs(value));
        if (previousMonths.length === 0) {
          return null;
        }
        const baseline = this.median(previousMonths);

        if (current === 0 && baseline === 0) {
          return null;
        }

        const delta = current - baseline;
        return {
          code: row.code,
          name: row.name,
          current,
          baseline,
          delta,
          deltaPercent: baseline > 0 ? (delta / baseline) * 100 : null,
          status: this.resolveCategoryBenchmarkStatus(current, baseline),
        };
      })
      .filter((row): row is CategoryBenchmarkRow => row !== null)
      .sort((left, right) => right.current - left.current);
  }

  private resolveCategoryBenchmarkStatus(current: number, baseline: number): CategoryBenchmarkStatus {
    if (baseline === 0) {
      return current > 0 ? 'above' : 'near';
    }

    if (current > baseline * 1.1) {
      return 'above';
    }
    if (current < baseline * 0.9) {
      return 'below';
    }
    return 'near';
  }

  private median(values: number[]): number {
    if (values.length === 0) {
      return 0;
    }

    const sorted = [...values].sort((left, right) => left - right);
    const middle = Math.floor(sorted.length / 2);
    return sorted.length % 2 === 0
      ? (sorted[middle - 1] + sorted[middle]) / 2
      : sorted[middle];
  }

  private cashflowMax(): number {
    return Math.max(...this.incomeMonthlyTotals(), ...this.expenseMonthlyTotalsAbsolute(), 1);
  }

  private expenseTrendMax(): number {
    return Math.max(...this.expenseTrendSeries().flatMap((series) => series.monthly), 1);
  }

  private minMax(values: number[]): { min: number; max: number } {
    const min = Math.min(...values, 0);
    const max = Math.max(...values, 0, 1);
    return { min, max };
  }

  private linePath(
    values: number[],
    width: number,
    height: number,
    top: number,
    right: number,
    bottom: number,
    max: number,
    min: number,
    left = top,
  ): string {
    if (values.length === 0) {
      return '';
    }
    const points = values.map((value, index) => ({
      x: this.pointX(index, width, left, right, values.length),
      y: this.pointY(value, height, top, bottom, max, min),
    }));
    return this.smoothPath(points);
  }

  private areaPath(
    values: number[],
    width: number,
    height: number,
    top: number,
    right: number,
    bottom: number,
    max: number,
    min: number,
    left = top,
  ): string {
    if (values.length === 0) {
      return '';
    }

    const baseline = this.pointY(min > 0 ? min : 0, height, top, bottom, max, min);
    const points = values.map((value, index) => ({
      x: this.pointX(index, width, left, right, values.length),
      y: this.pointY(value, height, top, bottom, max, min),
    }));
    const line = this.smoothPath(points);
    const lastX = this.pointX(values.length - 1, width, left, right, values.length);
    const firstX = this.pointX(0, width, left, right, values.length);
    return `${line} L ${lastX} ${baseline} L ${firstX} ${baseline} Z`;
  }

  private smoothPath(points: Array<{ x: number; y: number }>): string {
    if (points.length === 0) {
      return '';
    }

    if (points.length === 1) {
      const point = points[0];
      return `M ${point.x} ${point.y}`;
    }

    let path = `M ${points[0].x} ${points[0].y}`;

    for (let index = 0; index < points.length - 1; index += 1) {
      const current = points[index];
      const next = points[index + 1];
      const previous = points[index - 1] ?? current;
      const afterNext = points[index + 2] ?? next;
      const controlPoint1X = current.x + (next.x - previous.x) / 6;
      const controlPoint1Y = current.y + (next.y - previous.y) / 6;
      const controlPoint2X = next.x - (afterNext.x - current.x) / 6;
      const controlPoint2Y = next.y - (afterNext.y - current.y) / 6;

      path += ` C ${controlPoint1X} ${controlPoint1Y}, ${controlPoint2X} ${controlPoint2Y}, ${next.x} ${next.y}`;
    }

    return path;
  }

  private pointX(index: number, width: number, left: number, right: number, points: number): number {
    const usableWidth = width - left - right;
    const step = points > 1 ? usableWidth / (points - 1) : 0;
    return left + (index * step);
  }

  private pointY(value: number, height: number, top: number, bottom: number, max: number, min: number): number {
    const usableHeight = height - top - bottom;
    const domain = max - min || 1;
    return top + ((max - value) / domain) * usableHeight;
  }
}

interface ReportRow {
  code: string;
  name: string;
  monthly: number[];
  children: ReportRow[];
}

interface AccountSummary {
  code: string;
  name: string;
  type: 'ASSET' | 'LIABILITY';
  typeLabel: string;
  balance: number;
  width: number;
  color: string;
}

interface ExpenseBreakdownItem {
  code: string;
  name: string;
  amount: number;
  color: string;
  shareLabel: string;
  dashArray: string;
  dashOffset: string;
}

interface TrendSeries {
  code: string;
  name: string;
  monthly: number[];
  color: string;
}

type CategoryBenchmarkStatus = 'above' | 'near' | 'below';

interface CategoryBenchmarkRow {
  code: string;
  name: string;
  current: number;
  baseline: number;
  delta: number;
  deltaPercent: number | null;
  status: CategoryBenchmarkStatus;
}

const DONUT_COLORS = ['#1f8f68', '#3e78c2', '#d7644a', '#cf8b2c', '#7857b7', '#547a64'];
const TREND_COLORS = ['#1f8f68', '#3e78c2', '#d7644a', '#cf8b2c', '#6a5acd', '#2f9e44', '#b85c38', '#0f8b8d', '#c056a2', '#7a6f43'];
