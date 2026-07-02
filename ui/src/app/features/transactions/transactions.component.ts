import { Component, DestroyRef, ElementRef, HostListener, OnInit, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, ParamMap, Router } from '@angular/router';
import { CopyCheck, LucideAngularModule, SquarePen } from 'lucide-angular';
import { Observable, debounceTime } from 'rxjs';

import { ReferenceDataService } from '../../data/reference-data.service';
import { TransactionsService } from '../../data/transactions.service';
import { UserConfigService } from '../../data/user-config.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { uiMessages, deleteTransactionConfirmationMessage } from '../../shared/messages';
import { brazilianDateToQuery, centsToDecimal, dateInputToIso, decimalToCents, toBrazilianDate, toBrazilianDateInputValue } from '../../shared/money';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { Account, BulkTransactionUpdatePayload, Category, Pagination, Transaction, TransactionPayload, TransactionUpdatePayload } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

const TRANSACTION_PAGE_SIZE_MAX = 1000;
const TRANSACTION_OPERATION_OPTIONS = [
  { value: 'credit', labelKey: 'credit' },
  { value: 'debit', labelKey: 'debit' },
  { value: 'transfer', labelKey: 'transfer' },
] as const;

@Component({
  selector: 'app-transactions',
  imports: [ReactiveFormsModule, LucideAngularModule],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.page.eyebrow }}</p>
        <h1>{{ messages.page.title }}</h1>
      </div>
      <button
        class="ghost-button settings-button"
        type="button"
        [title]="messages.actions.settingsTitle"
        [attr.aria-label]="messages.actions.settingsAria"
        (click)="openSettings()"
      >
        <svg aria-hidden="true" viewBox="0 0 24 24">
          <path
            d="M19.14 12.94c.04-.31.06-.63.06-.94s-.02-.63-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.63l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.08 7.08 0 0 0-1.63-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.49.42l-.36 2.54c-.58.23-1.12.54-1.63.94l-2.39-.96a.5.5 0 0 0-.6.22L2.7 8.85a.5.5 0 0 0 .12.63l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94L2.82 14.52a.5.5 0 0 0-.12.63l1.92 3.32a.5.5 0 0 0 .6.22l2.39-.96c.5.4 1.05.71 1.63.94l.36 2.54a.5.5 0 0 0 .49.42h3.84a.5.5 0 0 0 .5-.42l.36-2.54c.58-.23 1.12-.54 1.63-.94l2.39.96a.5.5 0 0 0 .6-.22l1.92-3.32a.5.5 0 0 0-.12-.63l-2.03-1.58ZM12 15.5A3.5 3.5 0 1 1 12 8.5a3.5 3.5 0 0 1 0 7Z"
          />
        </svg>
      </button>
    </section>

    <section class="panel">
      <form class="filters transactions-filters" [formGroup]="filters">
        <label>
          {{ messages.filters.description }}
          <input formControlName="description" />
        </label>
        <div class="filter-field">
          <span class="filter-field-label">{{ messages.filters.account }}</span>
          <div class="multi-select" data-multi-select="account">
            <button class="multi-select-trigger" type="button" (click)="toggleFilterMenu('account')">
              <span class="multi-select-value">{{ selectedAccountLabel() }}</span>
              <span class="multi-select-caret" aria-hidden="true">{{ accountMenuOpen() ? '▴' : '▾' }}</span>
            </button>
            @if (accountMenuOpen()) {
              <div class="multi-select-menu">
                @for (group of accountFilterGroups(); track group.key) {
                  <div class="multi-select-group">
                    @if (group.label) {
                      <div class="multi-select-group-label">{{ group.label }}</div>
                    }
                    @for (account of group.options; track account.Code) {
                      <label class="multi-select-option">
                        <input
                          type="checkbox"
                          [checked]="isFilterSelected('account', account.Code)"
                          (change)="toggleFilterSelection('account', account.Code)"
                        />
                        <span>{{ accountName(account.Code) }}</span>
                      </label>
                    }
                  </div>
                }
              </div>
            }
          </div>
        </div>
        <div class="filter-field">
          <span class="filter-field-label">{{ messages.filters.category }}</span>
          <div class="multi-select" data-multi-select="category">
            <button class="multi-select-trigger" type="button" (click)="toggleFilterMenu('category')">
              <span class="multi-select-value">{{ selectedCategoryLabel() }}</span>
              <span class="multi-select-caret" aria-hidden="true">{{ categoryMenuOpen() ? '▴' : '▾' }}</span>
            </button>
            @if (categoryMenuOpen()) {
              <div class="multi-select-menu">
                @for (group of categoryFilterGroups(); track group.key) {
                  <div class="multi-select-group">
                    @if (group.label) {
                      <div class="multi-select-group-label">{{ group.label }}</div>
                    }
                    @for (category of group.options; track category.Code) {
                      <label class="multi-select-option">
                        <input
                          type="checkbox"
                          [checked]="isFilterSelected('category', category.Code)"
                          (change)="toggleFilterSelection('category', category.Code)"
                        />
                        <span>{{ category.Name }}</span>
                      </label>
                    }
                  </div>
                }
              </div>
            }
          </div>
        </div>
        <div class="filter-field">
          <span class="filter-field-label">{{ messages.filters.operation }}</span>
          <div class="multi-select" data-multi-select="operation">
            <button class="multi-select-trigger" type="button" (click)="toggleFilterMenu('operation')">
              <span class="multi-select-value">{{ selectedOperationLabel() }}</span>
              <span class="multi-select-caret" aria-hidden="true">{{ operationMenuOpen() ? '▴' : '▾' }}</span>
            </button>
            @if (operationMenuOpen()) {
              <div class="multi-select-menu">
                @for (option of operationOptions; track option.value) {
                  <label class="multi-select-option">
                    <input
                      type="checkbox"
                      [checked]="isFilterSelected('operation', option.value)"
                      (change)="toggleFilterSelection('operation', option.value)"
                    />
                    <span>{{ operationLabel(option.labelKey) }}</span>
                  </label>
                }
              </div>
            }
          </div>
        </div>
        <label>
          {{ messages.filters.from }}
          <input type="text" inputmode="numeric" [placeholder]="messages.filters.datePlaceholder" formControlName="from_date" />
        </label>
        <label>
          {{ messages.filters.to }}
          <input type="text" inputmode="numeric" [placeholder]="messages.filters.datePlaceholder" formControlName="to_date" />
        </label>
      </form>
    </section>

    <section class="panel">
      @if (loading()) {
        <div class="table-wrap">
          <div class="skeleton-table loading-shell transactions-skeleton" data-testid="transactions-skeleton">
            <div class="skeleton-table-header">
              <span class="skeleton skeleton-table-cell header"></span>
              <span class="skeleton skeleton-table-cell header"></span>
              <span class="skeleton skeleton-table-cell header"></span>
              <span class="skeleton skeleton-table-cell header"></span>
              <span class="skeleton skeleton-table-cell header"></span>
              <span class="skeleton skeleton-table-cell header"></span>
              <span class="skeleton skeleton-table-cell header"></span>
              <span class="skeleton skeleton-table-cell header actions"></span>
            </div>
            @for (row of tableSkeletonRows; track row) {
              <div class="skeleton-table-row">
                <span class="skeleton skeleton-table-cell"></span>
                <span class="skeleton skeleton-table-cell"></span>
                <span class="skeleton skeleton-table-cell"></span>
                <span class="skeleton skeleton-table-cell"></span>
                <span class="skeleton skeleton-table-cell"></span>
                <span class="skeleton skeleton-table-cell"></span>
                <span class="skeleton skeleton-table-cell"></span>
                <span class="skeleton skeleton-table-cell actions"></span>
              </div>
            }
          </div>
        </div>
        <div class="pagination-row">
          <span class="skeleton skeleton-pill"></span>
          <span class="skeleton skeleton-line short" style="width: 124px;"></span>
          <span class="skeleton skeleton-pill"></span>
        </div>
      } @else if (transactions().length === 0) {
        <p class="state-message">{{ messages.states.empty }}</p>
      } @else {
        @if (selectedCount() > 0) {
          <div class="bulk-actions-bar" data-testid="bulk-actions-overlay">
            <div class="bulk-actions-left">
              <button
                class="ghost-button bulk-icon-button"
                type="button"
                [title]="selectAllLabel()"
                [attr.aria-label]="selectAllLabel()"
                (click)="toggleSelectAllCurrentPage()"
              >
                @if (allCurrentPageSelected()) {
                  <svg aria-hidden="true" viewBox="0 0 24 24">
                    <path d="M5 5h14v14H5z" fill="none" stroke="currentColor" stroke-width="1.8"></path>
                    <path d="M8 8l8 8M16 8l-8 8" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="1.8"></path>
                  </svg>
                } @else {
                  <lucide-icon [img]="listChecksIcon" [size]="18" [strokeWidth]="1.9" aria-hidden="true" />
                }
              </button>
              <span class="bulk-actions-count">{{ selectedCountLabel() }}</span>
            </div>
            <button
              class="primary-button bulk-icon-button"
              type="button"
              [title]="messages.actions.editSelected"
              [attr.aria-label]="messages.actions.editSelected"
              (click)="openSelectedEdit()"
            >
              <lucide-icon [img]="editSelectedIcon" [size]="18" [strokeWidth]="1.9" aria-hidden="true" />
            </button>
          </div>
        }
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th class="selection-column">{{ messages.columns.select }}</th>
                <th>
                  <button class="sort-button" type="button" (click)="toggleSort('DATE')" [disabled]="loading()">
                    {{ messages.columns.date }} {{ sortIndicator('DATE') }}
                  </button>
                </th>
                <th>{{ messages.columns.description }}</th>
                <th>{{ messages.columns.category }}</th>
                <th>{{ messages.columns.account }}</th>
                <th>{{ messages.columns.transferAccount }}</th>
                <th>
                  <button class="sort-button" type="button" (click)="toggleSort('AMOUNT')" [disabled]="loading()">
                    {{ messages.columns.amount }} {{ sortIndicator('AMOUNT') }}
                  </button>
                </th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              @for (tx of transactions(); track tx.id) {
                <tr [class]="transactionRowClass(tx)" (click)="toggleTransactionSelection(tx.id)">
                  <td class="selection-cell">
                    <input
                      type="checkbox"
                      [checked]="isTransactionSelected(tx.id)"
                      [attr.aria-label]="messages.actions.selectAria"
                      (click)="$event.stopPropagation()"
                      (change)="toggleTransactionSelection(tx.id)"
                    />
                  </td>
                  <td class="date-cell">{{ dateLabel(tx.date) }}</td>
                  <td>{{ tx.description }}</td>
                  <td>{{ categoryName(tx.category_code) }}</td>
                  <td>{{ accountName(tx.account_code) }}</td>
                  <td>{{ transferAccountName(tx) }}</td>
                  <td class="amount-cell">{{ money(tx.amount) }}</td>
                  <td class="actions-cell">
                    <button class="icon-action" type="button" [title]="messages.actions.deleteTitle" [attr.aria-label]="messages.actions.deleteAria" (click)="$event.stopPropagation(); delete(tx)">
                      <svg aria-hidden="true" viewBox="0 0 24 24">
                        <path d="M9 3h6l1 2h4v2H4V5h4l1-2Zm-1 6h2v10H8V9Zm6 0h2v10h-2V9Zm-9 0h14l-1 12H6L5 9Z" />
                      </svg>
                    </button>
                  </td>
                </tr>
              }
            </tbody>
            @if (showPageTotal()) {
              <tfoot>
                <tr class="total-row">
                  <td colspan="6">{{ totalLabel() }}</td>
                  <td class="amount-cell">{{ money(displayedTotal()) }}</td>
                  <td></td>
                </tr>
              </tfoot>
            }
          </table>
        </div>
        @if (pagination()) {
          <div class="pagination-row">
            <button class="ghost-button" type="button" [disabled]="loading() || page() <= 1" (click)="setPage(page() - 1)">{{ messages.actions.previous }}</button>
            <span>{{ messages.actions.page }} {{ pagination()?.page }} {{ messages.actions.of }} {{ pagination()?.total_pages || 1 }}</span>
            <button class="ghost-button" type="button" [disabled]="loading() || page() >= (pagination()?.total_pages || 1)" (click)="setPage(page() + 1)">{{ messages.actions.next }}</button>
          </div>
        }
      }
    </section>

    @if (panelOpen()) {
      <aside class="side-panel">
        <div class="panel-header">
          <h2>{{ panelTitle() }}</h2>
          <button class="ghost-button" type="button" (click)="closePanel()">{{ messages.actions.close }}</button>
        </div>
        <form class="form-stack" [formGroup]="form" (ngSubmit)="save()">
          @if (!editingMany()) {
            <label>
              {{ messages.form.date }}
              <input type="text" inputmode="numeric" [placeholder]="messages.form.datePlaceholder" formControlName="date" />
            </label>
            <label>
              {{ messages.form.description }}
              <input formControlName="description" />
            </label>
            <label>
              {{ messages.form.amount }}
              <input type="text" inputmode="decimal" formControlName="amount" [placeholder]="messages.form.amountPlaceholder" />
            </label>
          }
          <label>
            {{ messages.form.account }}
            <select formControlName="account_code">
              @if (editingMany()) {
                <option value="">{{ messages.form.keepCurrent }}</option>
              }
              @for (account of referenceData.accounts(); track account.Code) {
                <option [value]="account.Code">{{ account.Name }}</option>
              }
            </select>
          </label>
          <label>
            {{ messages.form.category }}
            <select formControlName="category_code">
              @if (editingMany()) {
                <option value="">{{ messages.form.keepCurrent }}</option>
              }
              @for (group of formCategoryGroups(); track group.key) {
                @if (group.label) {
                  <optgroup [label]="group.label">
                    @for (category of group.options; track category.Code) {
                      <option [value]="category.Code">{{ category.Name }}</option>
                    }
                  </optgroup>
                } @else {
                  @for (category of group.options; track category.Code) {
                    <option [value]="category.Code">{{ category.Name }}</option>
                  }
                }
              }
            </select>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" formControlName="is_transfer" />
            {{ messages.form.transfer }}
          </label>
          @if (form.controls.is_transfer.value) {
            <label>
              {{ messages.form.transferAccount }}
              <select formControlName="account_transfer">
                <option value="">{{ editingMany() ? messages.form.keepCurrent : messages.form.selectAccount }}</option>
                @for (account of referenceData.accounts(); track account.Code) {
                  <option [value]="account.Code">{{ account.Name }}</option>
                }
              </select>
            </label>
          } @else {
            <label class="checkbox-label">
              <input type="checkbox" formControlName="exclude_from_dashboard" />
              <span>{{ messages.form.excludeFromDashboard }}</span>
            </label>
            <p class="field-hint">{{ messages.form.excludeFromDashboardHint }}</p>
          }
          <button class="primary-button" type="submit" [disabled]="!canSubmitForm()">
            {{ saving() ? messages.actions.saving : messages.actions.save }}
          </button>
        </form>
      </aside>
    }

    @if (settingsPanelOpen()) {
      <aside class="side-panel">
        <div class="panel-header">
          <div>
            <h2>{{ messages.settings.title }}</h2>
            <p>{{ messages.settings.description }}</p>
          </div>
          <button class="ghost-button" type="button" (click)="closeSettings()">{{ messages.actions.close }}</button>
        </div>
        <form class="form-stack" [formGroup]="settingsForm" (ngSubmit)="saveSettings()">
          <label>
            {{ messages.settings.pageSize }}
            <input type="number" min="1" [max]="maxPageSize" formControlName="page_size" />
            <small class="field-hint">{{ messages.settings.pageSizeHint }}</small>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" formControlName="show_total" />
            <span>{{ messages.settings.showTotal }}</span>
          </label>
          <p class="field-hint">{{ messages.settings.showTotalHint }}</p>
          <button class="primary-button" type="submit">{{ messages.actions.save }}</button>
        </form>
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

    .filter-field {
      display: grid;
      gap: 7px;
    }

    .filter-field-label {
      color: var(--muted);
      font-size: 0.86rem;
      font-weight: 700;
    }

    .multi-select {
      min-width: 0;
      position: relative;
    }

    .multi-select-trigger {
      align-items: center;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius-sm);
      color: inherit;
      cursor: pointer;
      display: flex;
      gap: 12px;
      justify-content: space-between;
      min-height: 46px;
      min-width: 0;
      overflow: hidden;
      padding: 12px 13px;
      text-align: left;
      width: 100%;
      box-shadow: inset 0 1px 0 rgba(21, 32, 29, 0.03);
    }

    .multi-select-trigger:focus-visible {
      outline: 2px solid rgba(24, 119, 103, 0.18);
      border-color: var(--accent);
    }

    .multi-select-value {
      display: block;
      flex: 1 1 auto;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .multi-select-caret {
      color: var(--muted);
      flex-shrink: 0;
      font-size: 12px;
    }

    .multi-select-menu {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      box-shadow: 0 18px 40px rgba(15, 23, 42, 0.14);
      display: grid;
      gap: 4px;
      left: 0;
      margin-top: 8px;
      max-height: 700px;
      min-width: max(100%, 320px);
      max-width: min(560px, calc(100vw - 32px));
      overflow-x: auto;
      overflow-y: auto;
      padding: 10px;
      position: absolute;
      scrollbar-color: #b7c7c0 transparent;
      scrollbar-width: thin;
      top: 100%;
      z-index: 20;
    }

    .multi-select-menu::-webkit-scrollbar {
      width: 10px;
    }

    .multi-select-menu::-webkit-scrollbar-thumb {
      background: #b7c7c0;
      border-radius: 999px;
      border: 2px solid transparent;
      background-clip: padding-box;
    }

    .multi-select-menu::-webkit-scrollbar-thumb:hover {
      background: #98ada5;
      background-clip: padding-box;
    }

    .multi-select-group + .multi-select-group {
      border-top: 1px solid var(--border);
      margin-top: 6px;
      padding-top: 10px;
    }

    .multi-select-group-label {
      color: var(--muted);
      font-size: 0.78rem;
      font-weight: 800;
      letter-spacing: 0.02em;
      padding: 4px 10px 6px;
      text-transform: uppercase;
    }

    .multi-select-option {
      align-items: center;
      border-radius: 10px;
      cursor: pointer;
      display: flex;
      gap: 10px;
      justify-content: flex-start;
      padding: 8px 10px;
      width: 100%;
    }

    .multi-select-option:hover {
      background: var(--surface-soft);
    }

    .multi-select-option input {
      appearance: auto;
      background: transparent;
      border: 0;
      flex: 0 0 auto;
      margin: 0;
      min-height: 0;
      padding: 0;
      width: auto;
      box-shadow: none;
    }

    .multi-select-option span {
      flex: 1 1 auto;
      min-width: 0;
      text-align: left;
      white-space: nowrap;
    }

    .total-row td {
      background: var(--surface-soft);
      font-weight: 600;
    }

    .transactions-skeleton {
      gap: 10px;
    }

    .selection-column,
    .selection-cell {
      text-align: center;
      width: 52px;
    }

    .selection-cell input {
      width: auto;
      margin: 0;
      box-shadow: none;
    }

    tbody tr {
      cursor: pointer;
    }

    .bulk-actions-bar {
      align-items: center;
      backdrop-filter: blur(10px);
      background: color-mix(in srgb, var(--surface) 86%, white 14%);
      border: 1px solid color-mix(in srgb, var(--border) 78%, var(--accent) 22%);
      border-radius: 999px;
      box-shadow: 0 18px 36px rgba(15, 23, 42, 0.16);
      display: flex;
      inset: auto 18px 18px auto;
      max-width: min(420px, calc(100vw - 36px));
      padding: 8px 10px;
      position: fixed;
      z-index: 30;
      align-items: center;
      gap: 12px;
      justify-content: space-between;
    }

    .bulk-actions-left {
      display: flex;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
      min-width: 0;
    }

    .bulk-actions-count {
      color: var(--muted);
      font-size: 0.9rem;
      font-weight: 700;
      min-width: 0;
    }

    .bulk-icon-button {
      flex: 0 0 auto;
      min-width: 40px;
      width: 40px;
      padding: 0;
    }

    .bulk-icon-button svg {
      width: 18px;
      height: 18px;
      fill: none;
      stroke: currentColor;
      stroke-linecap: round;
      stroke-linejoin: round;
      stroke-width: 1.8;
    }

    @media (max-width: 720px) {
      .bulk-actions-bar {
        border-radius: 18px;
        gap: 10px;
        inset: auto 12px 12px 12px;
        max-width: none;
        padding: 10px;
      }

      .bulk-actions-left {
        gap: 10px;
      }

      .bulk-actions-count {
        font-size: 0.84rem;
      }
    }
  `],
})
export class TransactionsComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly destroyRef = inject(DestroyRef);
  private readonly elementRef = inject(ElementRef<HTMLElement>);
  readonly messages = uiMessages.transactions;
  readonly commonMessages = uiMessages.common;
  readonly listChecksIcon = CopyCheck;
  readonly editSelectedIcon = SquarePen;
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly error = signal('');
  readonly transactions = signal<Transaction[]>([]);
  readonly pagination = signal<Pagination | null>(null);
  readonly panelOpen = signal(false);
  readonly settingsPanelOpen = signal(false);
  readonly editing = signal<Transaction | null>(null);
  readonly editMode = signal<EditMode>('create');
  readonly page = signal(1);
  readonly pageSize = signal(50);
  readonly showPageTotal = signal(false);
  readonly sortColumn = signal<TransactionSortColumn>('DATE');
  readonly sortDirection = signal<SortDirection>('desc');
  readonly accountMenuOpen = signal(false);
  readonly categoryMenuOpen = signal(false);
  readonly operationMenuOpen = signal(false);
  readonly maxPageSize = TRANSACTION_PAGE_SIZE_MAX;
  readonly tableSkeletonRows = [0, 1, 2, 3, 4, 5];
  readonly selectedTransactionIds = signal<string[]>([]);
  readonly displayedTotal = computed(() => {
    const source = this.selectedCount() > 0 ? this.selectedTransactions() : this.transactions();
    return source.reduce((sum, tx) => sum + tx.amount, 0);
  });
  readonly operationOptions = TRANSACTION_OPERATION_OPTIONS;
  readonly selectedTransactions = computed(() => {
    const selectedIds = new Set(this.selectedTransactionIds());
    return this.transactions().filter((tx) => selectedIds.has(tx.id));
  });
  readonly selectedCount = computed(() => this.selectedTransactions().length);
  readonly allCurrentPageSelected = computed(() =>
    this.transactions().length > 0 && this.transactions().every((tx) => this.selectedTransactionIds().includes(tx.id)),
  );
  readonly editingMany = computed(() => this.editMode() === 'edit-multi');
  readonly editingSingle = computed(() => this.editMode() === 'edit-single');
  private formBaseline: EditFormBaseline | null = null;

  readonly filters = this.fb.nonNullable.group({
    description: [''],
    account_codes: this.fb.nonNullable.control<string[]>([]),
    category_codes: this.fb.nonNullable.control<string[]>([]),
    operations: this.fb.nonNullable.control<TransactionOperationFilter[]>([]),
    from_date: [''],
    to_date: [''],
  });

  readonly form = this.fb.nonNullable.group({
    date: [toBrazilianDateInputValue(new Date()), [Validators.required, Validators.pattern(/^\d{2}\/\d{2}\/\d{4}$/)]],
    description: ['', Validators.required],
    amount: ['0,00', Validators.required],
    account_code: ['', Validators.required],
    category_code: ['', Validators.required],
    is_transfer: [false],
    account_transfer: [''],
    exclude_from_dashboard: [false],
  });

  readonly settingsForm = this.fb.nonNullable.group({
    page_size: [this.pageSize()],
    show_total: [this.showPageTotal()],
  });

  constructor(
    readonly referenceData: ReferenceDataService,
    private readonly transactionsService: TransactionsService,
    private readonly userConfigService: UserConfigService,
    private readonly moneyVisibility: MoneyVisibilityService,
    private readonly toast: ToastService,
    private readonly route: ActivatedRoute,
    private readonly router: Router,
  ) {
    const storedSettings = this.userConfigService.transactionListConfig();
    this.pageSize.set(this.clampPageSize(storedSettings.page_size));
    this.showPageTotal.set(storedSettings.show_total);
    this.settingsForm.patchValue({
      page_size: storedSettings.page_size,
      show_total: storedSettings.show_total,
    });

    this.filters.valueChanges
      .pipe(
        debounceTime(250),
        takeUntilDestroyed(),
      )
      .subscribe(() => {
        if (!this.canAutoApplyFilters()) {
          return;
        }
        this.applyFilters();
      });

    this.filters.controls.from_date.valueChanges
      .pipe(takeUntilDestroyed())
      .subscribe((value) => this.normalizeFilterDateControl('from_date', value));

    this.filters.controls.to_date.valueChanges
      .pipe(takeUntilDestroyed())
      .subscribe((value) => this.normalizeFilterDateControl('to_date', value));

    this.form.controls.date.valueChanges
      .pipe(takeUntilDestroyed())
      .subscribe((value) => this.normalizeFormDateControl(value));

    this.form.controls.is_transfer.valueChanges
      .pipe(takeUntilDestroyed())
      .subscribe(() => this.syncTransferFieldState());
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent): void {
    const target = event.target as HTMLElement | null;
    if (target?.closest('[data-multi-select="account"]')) {
      return;
    }
    if (target?.closest('[data-multi-select="category"]')) {
      return;
    }
    if (target?.closest('[data-multi-select="operation"]')) {
      return;
    }
    if (!this.elementRef.nativeElement.contains(target as Node | null)) {
      this.closeFilterMenus();
      return;
    }
    this.closeFilterMenus();
  }

  ngOnInit(): void {
    this.referenceData.load().subscribe({
      next: () => {
        this.route.queryParamMap
          .pipe(takeUntilDestroyed(this.destroyRef))
          .subscribe((params) => {
            this.applyFiltersFromRoute(params);
            this.page.set(1);
            this.loadTransactions();
          });
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
    });
  }

  loadTransactions(): void {
    this.loading.set(true);
    this.error.set('');
    this.clearSelection();
    const filters = this.filters.getRawValue();
    this.transactionsService.list({
      limit: this.pageSize(),
      page: this.page(),
      sort: this.sortParam(),
      description: filters.description,
      account_code: filters.account_codes,
      category_code: filters.category_codes,
      operation: filters.operations,
      from_date: brazilianDateToQuery(filters.from_date),
      to_date: brazilianDateToQuery(filters.to_date),
    }).subscribe({
      next: (response) => {
        this.transactions.set(response.transactions);
        this.pagination.set(response.pagination);
        this.userConfigService.syncTransactionListConfig(response.config);
        this.pageSize.set(this.clampPageSize(response.config.page_size));
        this.showPageTotal.set(response.config.show_total);
        this.settingsForm.patchValue({
          page_size: response.config.page_size,
          show_total: response.config.show_total,
        }, { emitEvent: false });
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }

  openEdit(tx: Transaction): void {
    this.selectedTransactionIds.set([tx.id]);
    this.openSelectedEdit();
  }

  openSelectedEdit(): void {
    const selected = this.selectedTransactions();
    if (selected.length === 0) {
      return;
    }
    this.settingsPanelOpen.set(false);
    this.error.set('');

    if (selected.length === 1) {
      this.openSingleEdit(selected[0]);
      return;
    }

    if (this.hasMixedTransferSelection(selected)) {
      this.toast.error(this.messages.actions.mixedSelectionError);
      return;
    }

    this.openMultiEdit(selected);
  }

  private openSingleEdit(tx: Transaction): void {
    this.settingsPanelOpen.set(false);
    this.editMode.set('edit-single');
    this.editing.set(tx);
    this.form.reset({
      date: toBrazilianDateInputValue(tx.date),
      description: tx.description,
      amount: centsToDecimal(tx.amount).replace('.', ','),
      account_code: tx.account_code,
      category_code: tx.category_code,
      is_transfer: Boolean(tx.account_transfer),
      account_transfer: tx.account_transfer ?? '',
      exclude_from_dashboard: tx.exclude_from_dashboard ?? false,
    });
    this.formBaseline = {
      date: toBrazilianDateInputValue(tx.date),
      description: tx.description,
      amount: tx.amount,
      account_code: tx.account_code,
      category_code: tx.category_code,
      is_transfer: Boolean(tx.account_transfer),
      account_transfer: tx.account_transfer ?? '',
      exclude_from_dashboard: tx.exclude_from_dashboard ?? false,
      mode: 'edit-single',
      transferBaseline: Boolean(tx.account_transfer),
    };
    this.configureFormForCurrentMode();
    this.panelOpen.set(true);
  }

  private openMultiEdit(transactions: Transaction[]): void {
    const areTransfers = transactions.every((tx) => this.isTransfer(tx));
    this.editMode.set('edit-multi');
    this.editing.set(null);
    this.form.reset({
      date: toBrazilianDateInputValue(new Date()),
      description: '',
      amount: '0,00',
      account_code: '',
      category_code: '',
      is_transfer: areTransfers,
      account_transfer: '',
      exclude_from_dashboard: false,
    });
    this.formBaseline = {
      date: '',
      description: '',
      amount: null,
      account_code: '',
      category_code: '',
      is_transfer: areTransfers,
      account_transfer: '',
      exclude_from_dashboard: false,
      mode: 'edit-multi',
      transferBaseline: areTransfers,
    };
    this.configureFormForCurrentMode();
    this.panelOpen.set(true);
  }

  closePanel(): void {
    this.panelOpen.set(false);
    this.editing.set(null);
    this.editMode.set('create');
    this.formBaseline = null;
  }

  openSettings(): void {
    this.closePanel();
    this.settingsForm.reset({
      page_size: this.pageSize(),
      show_total: this.showPageTotal(),
    });
    this.settingsPanelOpen.set(true);
  }

  closeSettings(): void {
    this.settingsPanelOpen.set(false);
  }

  saveSettings(): void {
    const value = this.settingsForm.getRawValue();
    const pageSize = this.clampPageSize(value.page_size);
    const showTotal = value.show_total;
    const pageSizeChanged = pageSize !== this.pageSize();
    this.error.set('');
    this.saving.set(true);

    this.userConfigService.updateTransactionListConfig({
      page_size: pageSize,
      show_total: showTotal,
    }).subscribe({
      next: () => {
        this.pageSize.set(pageSize);
        this.showPageTotal.set(showTotal);
        this.settingsForm.patchValue({
          page_size: pageSize,
          show_total: showTotal,
        }, { emitEvent: false });

        if (pageSizeChanged) {
          this.page.set(1);
          this.loadTransactions();
        }

        this.closeSettings();
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
      complete: () => this.saving.set(false),
    });
  }

  save(): void {
    if (!this.canSubmitForm()) {
      return;
    }
    this.saving.set(true);
    this.error.set('');
    const request = this.buildSaveRequest();
    if (!request) {
      this.saving.set(false);
      return;
    }

    request.subscribe({
      next: () => {
        this.closePanel();
        this.clearSelection();
        this.referenceData.reload().subscribe({
          error: (error) => this.toast.error(getApiErrorMessage(error)),
        });
        this.loadTransactions();
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.saving.set(false);
      },
      complete: () => this.saving.set(false),
    });
  }

  delete(tx: Transaction): void {
    if (!window.confirm(deleteTransactionConfirmationMessage(tx.description))) {
      return;
    }
    this.error.set('');
    this.transactionsService.delete(tx.id).subscribe({
      next: () => {
        this.referenceData.reload().subscribe({
          error: (error) => this.toast.error(getApiErrorMessage(error)),
        });
        this.loadTransactions();
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  setPage(page: number): void {
    this.page.set(page);
    this.loadTransactions();
  }

  toggleTransactionSelection(id: string): void {
    this.selectedTransactionIds.update((selected) =>
      selected.includes(id)
        ? selected.filter((current) => current !== id)
        : [...selected, id],
    );
  }

  isTransactionSelected(id: string): boolean {
    return this.selectedTransactionIds().includes(id);
  }

  toggleFilterMenu(type: FilterMenuType): void {
    if (type === 'account') {
      this.accountMenuOpen.update((open) => !open);
      this.categoryMenuOpen.set(false);
      this.operationMenuOpen.set(false);
      return;
    }
    if (type === 'category') {
      this.categoryMenuOpen.update((open) => !open);
      this.accountMenuOpen.set(false);
      this.operationMenuOpen.set(false);
      return;
    }

    this.operationMenuOpen.update((open) => !open);
    this.accountMenuOpen.set(false);
    this.categoryMenuOpen.set(false);
  }

  toggleFilterSelection(type: FilterMenuType, code: string): void {
    const control = type === 'account'
      ? this.filters.controls.account_codes
      : type === 'category'
        ? this.filters.controls.category_codes
        : this.filters.controls.operations;
    const next = control.value.includes(code)
      ? control.value.filter((current) => current !== code)
      : [...control.value, code];
    control.setValue(next);
  }

  isFilterSelected(type: FilterMenuType, code: string): boolean {
    const selected = type === 'account'
      ? this.filters.controls.account_codes.value
      : type === 'category'
        ? this.filters.controls.category_codes.value
        : this.filters.controls.operations.value;
    return selected.includes(code);
  }

  applyFilters(): void {
    this.page.set(1);
    this.syncRouteWithFilters();
  }

  toggleSort(column: TransactionSortColumn): void {
    if (this.sortColumn() === column) {
      this.sortDirection.update((direction) => direction === 'asc' ? 'desc' : 'asc');
    } else {
      this.sortColumn.set(column);
      this.sortDirection.set('asc');
    }
    this.page.set(1);
    this.loadTransactions();
  }

  sortIndicator(column: TransactionSortColumn): string {
    if (this.sortColumn() !== column) {
      return this.commonMessages.sortNeutral;
    }
    return this.sortDirection() === 'asc' ? this.commonMessages.sortAsc : this.commonMessages.sortDesc;
  }

  private sortParam(): string {
    const column = this.sortColumn().toLowerCase();
    return this.sortDirection() === 'desc' ? `-${column}` : column;
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrencyAbsolute(value);
  }

  dateLabel(value: string): string {
    return toBrazilianDate(value);
  }

  accountName(code: string): string {
    return this.referenceData.accountName(code);
  }

  categoryName(code: string): string {
    return this.referenceData.categoryName(code);
  }

  transferAccountName(tx: Transaction): string {
    return this.isTransfer(tx) ? this.referenceData.accountName(tx.account_transfer) : '';
  }

  isTransfer(tx: Transaction): boolean {
    return Boolean(tx.account_transfer && tx.transfer_id);
  }

  transactionRowClass(tx: Transaction): string {
    if (this.isTransfer(tx)) {
      return 'transfer-row';
    }
    return tx.amount >= 0 ? 'positive-row' : 'negative-row';
  }

  leafCategories() {
    const parentCodes = new Set(
      this.referenceData.activeCategories()
        .filter((category) => (category.SubCategories?.length ?? 0) > 0)
        .map((category) => category.Code),
    );
    return this.referenceData.activeFlatCategories().filter((category) => !parentCodes.has(category.Code));
  }

  selectedAccountLabel(): string {
    return this.selectedFilterLabel('account');
  }

  selectedCategoryLabel(): string {
    return this.selectedFilterLabel('category');
  }

  selectedOperationLabel(): string {
    return this.selectedFilterLabel('operation');
  }

  accountFilterGroups(): AccountFilterGroup[] {
    const active = this.referenceData.accounts().filter((account) => !account.DeactivatedAt);
    const inactive = this.referenceData.accounts().filter((account) => Boolean(account.DeactivatedAt));
    const groups: AccountFilterGroup[] = [];

    if (active.length > 0) {
      groups.push({
        key: 'active',
        label: null,
        options: active,
      });
    }
    if (inactive.length > 0) {
      groups.push({
        key: 'inactive',
        label: 'Desativadas',
        options: inactive,
      });
    }

    return groups;
  }

  formCategoryGroups(): CategoryFilterGroup[] {
    const includeMovement = this.form.controls.is_transfer.value;

    return this.categoryFilterGroups()
      .map((group) => ({
        ...group,
        options: group.options.filter((category) =>
          includeMovement ? category.Type === 'MOVEMENT' : category.Type !== 'MOVEMENT',
        ),
      }))
      .filter((group) => group.options.length > 0);
  }

  categoryFilterGroups(): CategoryFilterGroup[] {
    const groups: CategoryFilterGroup[] = [];

    for (const category of this.referenceData.activeCategories()) {
      const subCategories = category.SubCategories ?? [];
      if (subCategories.length > 0) {
        const selectableChildren = subCategories.filter((child) => (child.SubCategories?.length ?? 0) === 0);
        if (selectableChildren.length > 0) {
          groups.push({
            key: category.Code,
            label: category.Name,
            options: selectableChildren,
          });
        }
        continue;
      }

      groups.push({
        key: category.Code,
        label: null,
        options: [category],
      });
    }

    return groups;
  }

  private canAutoApplyFilters(): boolean {
    const { from_date, to_date } = this.filters.getRawValue();
    return this.isCompleteFilterDate(from_date) && this.isCompleteFilterDate(to_date);
  }

  private closeFilterMenus(): void {
    this.accountMenuOpen.set(false);
    this.categoryMenuOpen.set(false);
    this.operationMenuOpen.set(false);
  }

  private isCompleteFilterDate(value: string): boolean {
    return value === '' || /^\d{2}\/\d{2}\/\d{4}$/.test(value.trim());
  }

  private selectedFilterLabel(type: FilterMenuType): string {
    const selectedCodes = type === 'account'
      ? this.filters.controls.account_codes.value
      : type === 'category'
        ? this.filters.controls.category_codes.value
        : this.filters.controls.operations.value;
    if (selectedCodes.length === 0) {
      return type === 'operation' ? this.messages.filters.allOperations : this.messages.filters.all;
    }
    const names = selectedCodes.map((code) => {
      if (type === 'account') {
        return this.accountName(code);
      }
      if (type === 'category') {
        return this.categoryName(code);
      }
      return this.operationLabel(code as TransactionOperationFilter);
    });
    return names.join(', ');
  }

  operationLabel(operation: TransactionOperationFilter): string {
    return this.messages.filters.operationLabels[operation];
  }

  panelTitle(): string {
    if (this.editMode() === 'edit-multi') {
      return this.messages.form.editManyTitle;
    }
    if (this.editMode() === 'edit-single') {
      return this.messages.form.editTitle;
    }
    return this.messages.form.createTitle;
  }

  selectedEditLabel(): string {
    return this.messages.actions.editSelected;
  }

  selectAllLabel(): string {
    return this.allCurrentPageSelected() ? this.messages.actions.clearSelection : this.messages.actions.selectAll;
  }

  selectedCountLabel(): string {
    const count = this.selectedCount();
    return count === 1 ? '1 selecionada' : `${count} selecionadas`;
  }

  totalLabel(): string {
    return this.selectedCount() > 0
      ? this.messages.totals.selectedLabel
      : this.messages.totals.label;
  }

  canSubmitForm(): boolean {
    if (this.saving()) {
      return false;
    }
    if (this.editMode() === 'create') {
      return this.form.valid;
    }
    if (!this.formBaseline) {
      return false;
    }
    if (this.editMode() === 'edit-multi') {
      return this.form.valid && this.multiEditPayload() !== null;
    }
    if (this.editMode() === 'edit-single') {
      return this.form.valid && this.singleEditPayload() !== null;
    }
    return this.form.valid;
  }

  private clampPageSize(value: unknown): number {
    const numericValue = typeof value === 'number' ? value : Number(value);
    if (!Number.isFinite(numericValue)) {
      return 50;
    }
    return Math.min(this.maxPageSize, Math.max(1, Math.trunc(numericValue)));
  }

  private normalizeFilterDateControl(controlName: 'from_date' | 'to_date', value: string): void {
    const formatted = this.formatBrazilianDateInput(value);
    if (formatted === value) {
      return;
    }
    this.filters.controls[controlName].setValue(formatted);
  }

  private normalizeFormDateControl(value: string): void {
    const formatted = this.formatBrazilianDateInput(value);
    if (formatted === value) {
      return;
    }
    this.form.controls.date.setValue(formatted, { emitEvent: false });
  }

  private formatBrazilianDateInput(value: string): string {
    const digits = value.replace(/\D/g, '').slice(0, 8);
    if (digits.length <= 2) {
      return digits;
    }
    if (digits.length <= 4) {
      return `${digits.slice(0, 2)}/${digits.slice(2)}`;
    }
    return `${digits.slice(0, 2)}/${digits.slice(2, 4)}/${digits.slice(4)}`;
  }

  private applyFiltersFromRoute(params: ParamMap): void {
    const patch = {
      description: params.get('description') ?? '',
      account_codes: this.parseQueryList(params.get('account_code')),
      category_codes: this.parseQueryList(params.get('category_code')),
      operations: this.parseQueryList(params.get('operation')) as TransactionOperationFilter[],
      from_date: this.queryDateToFilterInput(params.get('from_date')),
      to_date: this.queryDateToFilterInput(params.get('to_date')),
    };

    this.filters.patchValue(patch, { emitEvent: false });
  }

  private syncRouteWithFilters(): void {
    const queryParams = this.buildFilterQueryParams();
    if (this.filterQueryMatches(this.route.snapshot.queryParamMap, queryParams)) {
      this.loadTransactions();
      return;
    }

    this.router.navigate([], {
      relativeTo: this.route,
      queryParams,
      replaceUrl: true,
    });
  }

  private buildFilterQueryParams(): Record<string, string | null> {
    const filters = this.filters.getRawValue();
    return {
      description: filters.description.trim() || null,
      account_code: this.serializeFilterQueryList(filters.account_codes),
      category_code: this.serializeFilterQueryList(filters.category_codes),
      operation: this.serializeFilterQueryList(filters.operations),
      from_date: brazilianDateToQuery(filters.from_date) || null,
      to_date: brazilianDateToQuery(filters.to_date) || null,
    };
  }

  private filterQueryMatches(params: ParamMap, queryParams: Record<string, string | null>): boolean {
    return Object.entries(queryParams).every(([key, value]) => (params.get(key) ?? null) === value);
  }

  private parseQueryList(value: string | null): string[] {
    if (!value) {
      return [];
    }

    return value
      .split(',')
      .map((item) => item.trim())
      .filter((item) => item.length > 0);
  }

  private queryDateToFilterInput(value: string | null): string {
    if (!value) {
      return '';
    }

    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value.trim());
    if (!match) {
      return value;
    }

    const [, year, month, day] = match;
    return `${day}/${month}/${year}`;
  }

  private serializeFilterQueryList(values: string[]): string | null {
    return values.length > 0 ? values.join(',') : null;
  }

  private buildSaveRequest(): Observable<unknown> | null {
    if (this.editMode() === 'edit-single' && this.editing()) {
      const payload = this.singleEditPayload();
      if (!payload) {
        return null;
      }
      return this.transactionsService.update(this.editing()!.id, payload);
    }

    if (this.editMode() === 'edit-multi') {
      const payload = this.multiEditPayload();
      if (!payload) {
        return null;
      }
      return this.transactionsService.updateMany(payload);
    }

    if (this.form.invalid) {
      return null;
    }

    const payload = this.fullTransactionPayload();
    return this.transactionsService.create(payload);
  }

  private fullTransactionPayload(): TransactionPayload {
    const value = this.form.getRawValue();
    return {
      date: dateInputToIso(brazilianDateToQuery(value.date)),
      description: value.description,
      amount: decimalToCents(value.amount),
      account_code: value.account_code,
      category_code: value.category_code,
      is_transfer: value.is_transfer,
      account_transfer: value.is_transfer ? value.account_transfer || null : null,
      exclude_from_dashboard: value.is_transfer ? false : value.exclude_from_dashboard,
    };
  }

  private singleEditPayload(): TransactionUpdatePayload | null {
    const tx = this.editing();
    if (!tx) {
      return null;
    }

    const baseline = {
      date: toBrazilianDateInputValue(tx.date),
      description: tx.description,
      amount: tx.amount,
      account_code: tx.account_code,
      category_code: tx.category_code,
      is_transfer: this.isTransfer(tx),
      account_transfer: tx.account_transfer ?? '',
      exclude_from_dashboard: tx.exclude_from_dashboard ?? false,
    };
    const current = this.fullTransactionPayload();
    const payload: TransactionUpdatePayload = {};

    if (dateInputToIso(brazilianDateToQuery(baseline.date)) !== current.date) {
      payload.date = current.date;
    }
    if (baseline.description !== current.description) {
      payload.description = current.description;
    }
    if (baseline.amount !== current.amount) {
      payload.amount = current.amount;
    }
    if (baseline.account_code !== current.account_code) {
      payload.account_code = current.account_code;
    }
    if (baseline.category_code !== current.category_code) {
      payload.category_code = current.category_code;
    }
    if (baseline.is_transfer !== current.is_transfer) {
      payload.is_transfer = current.is_transfer;
    }
    const baselineTransferAccount = baseline.account_transfer || null;
    const currentTransferAccount = current.account_transfer || null;
    if (baselineTransferAccount !== currentTransferAccount || baseline.is_transfer !== current.is_transfer) {
      payload.account_transfer = current.account_transfer ?? null;
    }
    if (baseline.exclude_from_dashboard !== current.exclude_from_dashboard) {
      payload.exclude_from_dashboard = current.exclude_from_dashboard;
    }

    return Object.keys(payload).length > 0 ? payload : null;
  }

  private multiEditPayload(): BulkTransactionUpdatePayload | null {
    const selectedIds = this.selectedTransactions().map((tx) => tx.id);
    if (selectedIds.length < 2 || !this.formBaseline) {
      return null;
    }

    const value = this.form.getRawValue();
    const payload: BulkTransactionUpdatePayload = { ids: selectedIds };
    const baseline = this.formBaseline;

    if (value.account_code) {
      payload.account_code = value.account_code;
    }
    if (value.category_code) {
      payload.category_code = value.category_code;
    }

    if (value.is_transfer !== baseline.is_transfer) {
      payload.is_transfer = value.is_transfer;
      if (!value.is_transfer) {
        payload.account_transfer = null;
      }
    }

    if (value.is_transfer) {
      if (!baseline.transferBaseline && !value.account_transfer) {
        return null;
      }
      if (value.account_transfer) {
        payload.account_transfer = value.account_transfer;
      }
    }

    if (!value.is_transfer && this.form.controls.exclude_from_dashboard.dirty) {
      payload.exclude_from_dashboard = value.exclude_from_dashboard;
    }

    return Object.keys(payload).length > 1 ? payload : null;
  }

  private configureFormForCurrentMode(): void {
    const multi = this.editMode() === 'edit-multi';
    this.toggleControlState('date', !multi);
    this.toggleControlState('description', !multi);
    this.toggleControlState('amount', !multi);

    if (multi) {
      this.form.controls.account_code.clearValidators();
      this.form.controls.category_code.clearValidators();
      this.form.controls.date.clearValidators();
      this.form.controls.description.clearValidators();
      this.form.controls.amount.clearValidators();
    } else {
      this.form.controls.account_code.setValidators([Validators.required]);
      this.form.controls.category_code.setValidators([Validators.required]);
      this.form.controls.date.setValidators([Validators.required]);
      this.form.controls.description.setValidators([Validators.required]);
      this.form.controls.amount.setValidators([Validators.required]);
    }

    this.syncTransferFieldState();
    this.form.controls.account_code.updateValueAndValidity({ emitEvent: false });
    this.form.controls.category_code.updateValueAndValidity({ emitEvent: false });
    this.form.controls.date.updateValueAndValidity({ emitEvent: false });
    this.form.controls.description.updateValueAndValidity({ emitEvent: false });
    this.form.controls.amount.updateValueAndValidity({ emitEvent: false });
  }

  private toggleControlState(controlName: 'date' | 'description' | 'amount', enabled: boolean): void {
    const control = this.form.controls[controlName];
    if (enabled) {
      control.enable({ emitEvent: false });
      return;
    }
    control.disable({ emitEvent: false });
  }

  private syncTransferFieldState(): void {
    const transferControl = this.form.controls.account_transfer;
    const excludeControl = this.form.controls.exclude_from_dashboard;
    const baselineTransfer = this.formBaseline?.transferBaseline ?? false;
    if (this.form.controls.is_transfer.value) {
      transferControl.enable({ emitEvent: false });
      if (this.editMode() === 'edit-multi') {
        transferControl.setValidators(!baselineTransfer ? [Validators.required] : []);
      } else {
        transferControl.setValidators([Validators.required]);
      }
      excludeControl.setValue(false, { emitEvent: false });
      excludeControl.disable({ emitEvent: false });
    } else {
      transferControl.clearValidators();
      transferControl.setValue('', { emitEvent: false });
      if (this.editMode() === 'edit-multi') {
        transferControl.enable({ emitEvent: false });
      } else {
        transferControl.disable({ emitEvent: false });
      }
      excludeControl.enable({ emitEvent: false });
    }
    transferControl.updateValueAndValidity({ emitEvent: false });
    excludeControl.updateValueAndValidity({ emitEvent: false });
    this.syncCategoryFieldState();
  }

  private syncCategoryFieldState(): void {
    const control = this.form.controls.category_code;
    const selectedCategory = control.value;
    if (!selectedCategory) {
      return;
    }

    const availableCategories = this.formCategoryGroups().flatMap((group) => group.options);
    if (!availableCategories.some((category) => category.Code === selectedCategory)) {
      control.setValue('', { emitEvent: false });
    }
  }

  private hasMixedTransferSelection(transactions: Transaction[]): boolean {
    const transferCount = transactions.filter((tx) => this.isTransfer(tx)).length;
    return transferCount > 0 && transferCount < transactions.length;
  }

  toggleSelectAllCurrentPage(): void {
    if (this.allCurrentPageSelected()) {
      this.clearSelection();
      return;
    }
    this.selectedTransactionIds.set(this.transactions().map((tx) => tx.id));
  }

  private clearSelection(): void {
    this.selectedTransactionIds.set([]);
  }
}

type TransactionSortColumn = 'DATE' | 'AMOUNT';
type SortDirection = 'asc' | 'desc';
type FilterMenuType = 'account' | 'category' | 'operation';
type TransactionOperationFilter = 'credit' | 'debit' | 'transfer';
type AccountFilterGroup = {
  key: string;
  label: string | null;
  options: Account[];
};
type CategoryFilterGroup = {
  key: string;
  label: string | null;
  options: Category[];
};

type EditMode = 'create' | 'edit-single' | 'edit-multi';

type EditFormBaseline = {
  date: string;
  description: string;
  amount: number | null;
  account_code: string;
  category_code: string;
  is_transfer: boolean;
  account_transfer: string;
  exclude_from_dashboard: boolean;
  mode: EditMode;
  transferBaseline: boolean;
};
