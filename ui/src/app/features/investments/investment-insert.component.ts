import { AfterViewInit, Component, DestroyRef, ElementRef, OnInit, computed, effect, inject, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { catchError, debounce, distinctUntilChanged, forkJoin, of, Subject, switchMap, timer } from 'rxjs';

import { InvestmentsService } from '../../data/investments.service';
import { ReferenceDataService } from '../../data/reference-data.service';
import { TransactionsService } from '../../data/transactions.service';
import { UserConfigService } from '../../data/user-config.service';
import { getApiErrorCode, getApiErrorDetails, getApiErrorMessage } from '../../shared/api-error';
import { uiMessages } from '../../shared/messages';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { brazilianDateToQuery, centsToCurrency, centsToDecimal, dateInputToIso, decimalToCents } from '../../shared/money';
import {
  Account,
  ImportInvestmentOperationsPayload,
  InvestmentMirrorExtraType,
  InvestmentMirrorPreviewRow,
  InvestmentOperationType,
  InvestmentPositionPreviewRow,
} from '../../shared/models';
import { ToastService } from '../../shared/toast.service';
import { investmentAssetLabel } from '../../shared/labels';

interface DraftOperationRow {
  id: number;
  date: string;
  assetCode: string;
  operationType: InvestmentOperationType | '';
  brokerageAccountCode: string;
  investmentAccountCode: string;
  quantity: string;
  unitPrice: string;
  unitPriceManualDecimal: boolean;
  totalFeeAmount: string;
  totalFeeAmountManualDecimal: boolean;
}

interface RowValidation {
  valid: boolean;
  errors: string[];
}

interface MirroredDraftRow {
  clientRowId: string;
  groupKey: string;
  operationType: InvestmentOperationType;
  brokerageAccountCode: string;
  investmentAccountCode: string;
  date: string;
  description: string;
  transferAmount: number;
  extraAmount: number;
  extraType: InvestmentMirrorExtraType;
  sourceAccountCode: string;
  destinationAccountCode: string;
  transactionId: string;
  attachExtraTransaction: boolean;
  extraTransactionId: string;
}

type MirrorMode = 'create' | 'attach';
type MirrorCandidate = {
  id: string;
  amount: number;
  label: string;
  date: string;
  accountCode: string;
  transferAccountCode: string;
  categoryCode?: string;
};
type MirrorCandidateGroup = { label: string; items: MirrorCandidate[] };
type PreviewRequest = {
  signature: string;
  payload: ImportInvestmentOperationsPayload | null;
  immediate: boolean;
  blocked?: boolean;
};
type PreviewResult = {
  position_preview_rows: InvestmentPositionPreviewRow[];
  mirror_preview_rows?: InvestmentMirrorPreviewRow[];
  clear: boolean;
  blocked: boolean;
  rowErrors?: Record<number, string[]>;
  message?: string | null;
};

const INITIAL_ROWS = 10;
const INSERT_COLUMN_COUNT = 8;
const MIRROR_OPTION_LABEL_MAX_CHARS = 67;
const MIRROR_OPTION_DATE_CHARS = 8;
const MIRROR_OPTION_SEPARATOR = ' · ';
const MIRROR_OPTION_AMOUNT_MAX_CHARS = 12;
const MIRROR_OPTION_DESCRIPTION_MAX_CHARS =
  MIRROR_OPTION_LABEL_MAX_CHARS
  - MIRROR_OPTION_DATE_CHARS
  - (MIRROR_OPTION_SEPARATOR.length * 2)
  - MIRROR_OPTION_AMOUNT_MAX_CHARS;

@Component({
  selector: 'app-investment-insert',
  imports: [FormsModule, RouterLink, RouterLinkActive],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.page.eyebrow }}</p>
        <h1>{{ messages.page.title }}</h1>
        <p class="page-subtitle">{{ messages.page.subtitle }}</p>
      </div>
      <div class="insert-actions">
        <button class="ghost-button" type="button" (click)="addRows(10)">{{ messages.page.addRows }}</button>
        <button class="ghost-button" type="button" (click)="clearEmptyRows()">{{ messages.page.clearEmpty }}</button>
        <button class="primary-button" type="button" [disabled]="!canSubmit() || saving()" (click)="submit()">
          {{ saving() ? messages.page.submitting : messages.page.submit }}
        </button>
      </div>
    </section>

    <nav class="panel investment-subnav">
      <a routerLink="/investments/dashboard" routerLinkActive="active">{{ nav.dashboard }}</a>
      <a routerLink="/investments/positions" routerLinkActive="active">{{ nav.positions }}</a>
      <a routerLink="/investments/assets" routerLinkActive="active">{{ nav.assets }}</a>
      <a routerLink="/investments/insert" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }">
        {{ nav.insert }}
      </a>
      <a routerLink="/investments/operations" routerLinkActive="active">{{ nav.operations }}</a>
      <a routerLink="/investments/portfolios" routerLinkActive="active">{{ nav.portfolios }}</a>
    </nav>

    @if (!sellAutomationConfigured()) {
      <section class="panel page-warning">
        <strong>{{ messages.sellAutomationWarning.title }}</strong>
        <p>{{ messages.sellAutomationWarning.body }}</p>
      </section>
    }

    <section class="panel">
      <div class="insert-table-shell">
        <div class="table-wrap insert-table-wrap">
          <table class="insert-table investment-insert-table">
            <colgroup>
              <col class="insert-col-date investment-insert-col-date" />
              <col class="insert-col-description investment-insert-col-asset" />
              <col class="insert-col-type investment-insert-col-type" />
              <col class="insert-col-description investment-insert-col-account" />
              <col class="insert-col-description investment-insert-col-account" />
              <col class="insert-col-amount" />
              <col class="insert-col-amount" />
              <col class="insert-col-amount" />
              <col class="insert-col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>{{ messages.columns.date }}</th>
                <th>{{ messages.columns.asset }}</th>
                <th>{{ messages.columns.type }}</th>
                <th>{{ messages.columns.brokerageAccount }}</th>
                <th>{{ messages.columns.investmentAccount }}</th>
                <th>{{ messages.columns.quantity }}</th>
                <th>{{ messages.columns.unitPrice }}</th>
                <th>{{ messages.columns.totalFee }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              @for (row of rows(); track row.id; let rowIndex = $index) {
                <tr [class.invalid-draft-row]="rowValidation(row).errors.length > 0">
                  <td>
                    <input
                      class="grid-input compact-grid-input"
                      data-grid-cell
                      inputmode="numeric"
                      [placeholder]="messages.placeholders.date"
                      [ngModel]="dateInputValue(row)"
                      name="date-{{ row.id }}"
                      (focus)="startDateEdit(row, $any($event.target))"
                      (ngModelChange)="onDateInput(row, $event)"
                      (blur)="finishDateEdit(row); requestPreviewNow()"
                      (paste)="handlePaste($event, rowIndex, 0)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td>
                    <input
                      class="grid-input"
                      data-grid-cell
                      [placeholder]="messages.placeholders.asset"
                      [ngModel]="row.assetCode"
                      name="asset-{{ row.id }}"
                      (ngModelChange)="updateAssetCode(row, $event)"
                      (blur)="normalizeAsset(row); requestPreviewNow()"
                      (paste)="handlePaste($event, rowIndex, 1)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td>
                    <select
                      class="grid-input compact-grid-input"
                      data-grid-cell
                      [ngModel]="row.operationType"
                      (ngModelChange)="onOperationTypeChange(row, $event)"
                      name="type-{{ row.id }}"
                      (paste)="handlePaste($event, rowIndex, 2)"
                      (keydown)="moveCell($event)"
                    >
                      <option value=""></option>
                      <option value="BUY">{{ messages.types.buy }}</option>
                      <option value="SELL">{{ messages.types.sell }}</option>
                      <option value="BONIFICATION">{{ messages.types.bonification }}</option>
                      <option value="AMORTIZATION">{{ messages.types.amortization }}</option>
                    </select>
                  </td>
                  <td>
                    <select
                      class="grid-input compact-grid-input"
                      data-grid-cell
                      [ngModel]="row.brokerageAccountCode"
                      (ngModelChange)="onBrokerageAccountCodeChange(row, $event)"
                      name="brokerage-account-{{ row.id }}"
                      (paste)="handlePaste($event, rowIndex, 3)"
                      (keydown)="moveCell($event)"
                    >
                      <option value=""></option>
                      @for (account of brokerageAccounts(); track account.Code) {
                        <option [value]="account.Code">{{ account.Name }}</option>
                      }
                    </select>
                  </td>
                  <td>
                    <select
                      class="grid-input compact-grid-input"
                      data-grid-cell
                      [ngModel]="row.investmentAccountCode"
                      (ngModelChange)="onInvestmentAccountCodeChange(row, $event)"
                      name="investment-account-{{ row.id }}"
                      (paste)="handlePaste($event, rowIndex, 4)"
                      (keydown)="moveCell($event)"
                    >
                      <option value=""></option>
                      @for (account of investmentAccounts(); track account.Code) {
                        <option [value]="account.Code">{{ account.Name }}</option>
                      }
                    </select>
                  </td>
                  <td>
                    <input
                      class="grid-input compact-grid-input"
                      data-grid-cell
                      [placeholder]="messages.placeholders.quantity"
                      inputmode="numeric"
                      [ngModel]="row.quantity"
                      (ngModelChange)="updateQuantity(row, $event)"
                      name="quantity-{{ row.id }}"
                      (blur)="normalizeQuantity(row); requestPreviewNow()"
                      (paste)="handlePaste($event, rowIndex, 5)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td>
                    <input
                      class="grid-input amount-draft-input"
                      data-grid-cell
                      [placeholder]="messages.placeholders.unitPrice"
                      inputmode="decimal"
                      [ngModel]="row.unitPrice"
                      name="unit-price-{{ row.id }}"
                      (ngModelChange)="onMoneyInput(row, 'unitPrice', $event)"
                      (blur)="finishMoneyEdit(row, 'unitPrice'); requestPreviewNow()"
                      (paste)="handlePaste($event, rowIndex, 6)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td>
                    <input
                      class="grid-input amount-draft-input"
                      data-grid-cell
                      [placeholder]="messages.placeholders.totalFee"
                      inputmode="decimal"
                      [ngModel]="row.totalFeeAmount"
                      name="total-fee-{{ row.id }}"
                      (ngModelChange)="onMoneyInput(row, 'totalFeeAmount', $event)"
                      (blur)="finishMoneyEdit(row, 'totalFeeAmount'); requestPreviewNow()"
                      (paste)="handlePaste($event, rowIndex, 7)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td class="actions-cell investment-insert-actions-cell">
                    <div class="investment-insert-actions">
                      <button class="icon-action" type="button" tabindex="-1" [title]="messages.actions.duplicateTitle" [attr.aria-label]="messages.actions.duplicateAria" (click)="duplicateRow(rowIndex)">⧉</button>
                      <button class="icon-action" type="button" tabindex="-1" [title]="messages.actions.addBelowTitle" [attr.aria-label]="messages.actions.addBelowAria" (click)="addRowAfter(rowIndex)">+</button>
                      <button class="icon-action" type="button" tabindex="-1" [title]="messages.actions.clearTitle" [attr.aria-label]="messages.actions.clearAria" (click)="clearRow(row)">×</button>
                    </div>
                  </td>
                </tr>
                @if (rowValidation(row).errors.length > 0) {
                  <tr class="draft-error-row">
                    <td colspan="9">{{ rowValidation(row).errors.join(' · ') }}</td>
                  </tr>
                }
              }
            </tbody>
          </table>
        </div>

        @if (positionPreviewRows().length > 0 && !previewBlocked()) {
          <div class="insert-preview-block">
            <div class="panel-header insert-preview-header">
              <h2>{{ messages.preview.title }}</h2>
            </div>
            <div class="table-wrap insert-preview-wrap">
              <table class="insert-preview-table">
                <thead>
                  <tr>
                    <th class="insert-preview-asset-column">{{ messages.preview.asset }}</th>
                    <th>{{ messages.preview.currentQuantity }}</th>
                    <th>{{ messages.preview.draftChange }}</th>
                    <th>{{ messages.preview.projectedQuantity }}</th>
                    <th>{{ messages.preview.currentAveragePrice }}</th>
                    <th>{{ messages.preview.projectedAveragePrice }}</th>
                  </tr>
                </thead>
                <tbody>
                  @for (row of positionPreviewRows(); track row.asset_code) {
                    <tr>
                      <td class="insert-preview-asset-column">{{ assetLabel(row.asset_code, row.asset_name) }}</td>
                      <td class="amount-cell">{{ row.current_quantity }}</td>
                      <td class="amount-cell">{{ row.draft_change }}</td>
                      <td class="amount-cell"><strong>{{ row.projected_quantity }}</strong></td>
                      <td class="amount-cell">{{ money(row.current_average_price) }}</td>
                      <td class="amount-cell"><strong>{{ money(row.projected_average_price) }}</strong></td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          </div>
        } @else if (previewBlocked()) {
          <div class="insert-preview-block">
            <div class="panel-header insert-preview-header">
              <h2>{{ messages.preview.title }}</h2>
            </div>
            <div class="panel page-warning insert-preview-state">
              <strong>{{ messages.states.previewBlocked }}</strong>
              <p>{{ previewBlockedHint() || messages.states.previewBlockedHint }}</p>
            </div>
          </div>
        }
      </div>
    </section>

    @if (mirrorModalOpen()) {
      <div class="modal-backdrop" (click)="cancelMirrorModal()">
        <section class="panel mirror-modal" (click)="$event.stopPropagation()">
          <div class="panel-header mirror-modal-header">
            <div>
              <h2>{{ messages.mirror.title }}</h2>
              <p class="page-subtitle">{{ messages.mirror.subtitle }}</p>
            </div>
          </div>
          <div class="mirror-mode-switch">
            <label class="checkbox-label">
              <input type="radio" name="insert-mirror-mode" [checked]="mirrorMode() === 'create'" (change)="mirrorMode.set('create')" />
              <span>{{ messages.mirror.createMode }}</span>
            </label>
            <label class="checkbox-label">
              <input type="radio" name="insert-mirror-mode" [checked]="mirrorMode() === 'attach'" (change)="mirrorMode.set('attach')" />
              <span>{{ messages.mirror.attachMode }}</span>
            </label>
          </div>
          @if (mirrorRows().length > 1) {
            <p class="field-hint">{{ messages.mirror.applyAllHint }}</p>
          }
          <div class="table-wrap insert-table-wrap">
            <table class="insert-preview-table" [class.mirror-table-attach]="mirrorMode() === 'attach'">
              <thead>
                <tr>
                  <th>{{ messages.columns.date }}</th>
                  <th>{{ messages.mirror.description }}</th>
                  <th>{{ messages.mirror.summary }}</th>
                  @if (mirrorMode() === 'create') {
                    <th>{{ messages.mirror.source }}</th>
                    <th>{{ messages.mirror.destination }}</th>
                  } @else {
                    <th>{{ messages.mirror.existingTransaction }}</th>
                    <th>{{ messages.mirror.extraTransaction }}</th>
                  }
                </tr>
              </thead>
              <tbody>
                @for (row of mirrorRows(); track row.clientRowId) {
                  <tr [class.mirror-row-buy]="row.operationType === 'BUY'" [class.mirror-row-sell]="row.operationType === 'SELL'" [class.mirror-row-bonification]="row.operationType === 'BONIFICATION'">
                    <td>{{ mirrorDisplayDate(row.date) }}</td>
                    <td>{{ row.description }}</td>
                    <td class="mirror-summary-cell">
                      @for (line of mirrorSummaryLines(row); track line) {
                        <div>{{ line }}</div>
                      }
                    </td>
                    @if (mirrorMode() === 'create') {
                      <td>
                        <select
                          class="grid-input compact-grid-input"
                          [ngModel]="row.sourceAccountCode"
                          name="mirror-source-{{ row.clientRowId }}"
                          (ngModelChange)="updateMirrorAccount(row, 'sourceAccountCode', $event)"
                        >
                          <option value=""></option>
                          @for (account of activeAccounts(); track account.Code) {
                            <option [value]="account.Code">{{ account.Name }}</option>
                          }
                        </select>
                      </td>
                      <td>
                        <select
                          class="grid-input compact-grid-input"
                          [ngModel]="row.destinationAccountCode"
                          name="mirror-destination-{{ row.clientRowId }}"
                          (ngModelChange)="updateMirrorAccount(row, 'destinationAccountCode', $event)"
                        >
                          <option value=""></option>
                          @for (account of activeAccounts(); track account.Code) {
                            <option [value]="account.Code">{{ account.Name }}</option>
                          }
                        </select>
                      </td>
                    } @else {
                      <td class="mirror-transaction-cell">
                        <select
                          class="grid-input compact-grid-input"
                          [ngModel]="row.transactionId"
                          name="mirror-transaction-{{ row.clientRowId }}"
                          (ngModelChange)="updateMirrorTransaction(row, $event)"
                        >
                          <option value=""></option>
                          @for (group of mirrorCandidateGroups(row); track group.label) {
                            <optgroup [label]="group.label">
                              @for (candidate of group.items; track $index) {
                                <option [value]="candidate.id">{{ candidate.label }}</option>
                              }
                            </optgroup>
                          }
                        </select>
                        @if (mirrorBrokerageHint(row); as hint) {
                          <div class="mirror-inline-hint">{{ hint }}</div>
                        }
                      </td>
                      <td class="mirror-pnl-cell">
                        @if (row.extraType !== 'NONE') {
                          <label class="checkbox-label mirror-checkbox">
                            <input
                              type="checkbox"
                              [ngModel]="row.attachExtraTransaction"
                              name="mirror-pnl-toggle-{{ row.clientRowId }}"
                              (ngModelChange)="toggleMirrorExtraTransaction(row, $event)"
                            />
                            <span>{{ row.extraType === 'BONIFICATION_INCOME' ? messages.mirror.attachBonificationIncome : messages.mirror.attachRealizedPnl }}</span>
                          </label>
                          @if (row.attachExtraTransaction) {
                            <select
                              class="grid-input compact-grid-input"
                              [ngModel]="row.extraTransactionId"
                              name="mirror-pnl-transaction-{{ row.clientRowId }}"
                              (ngModelChange)="updateMirrorExtraTransaction(row, $event)"
                            >
                              <option value=""></option>
                              @for (group of mirrorPnlCandidateGroups(row); track group.label) {
                                <optgroup [label]="group.label">
                                  @for (candidate of group.items; track candidate.id) {
                                    <option [value]="candidate.id">{{ candidate.label }}</option>
                                  }
                                </optgroup>
                              }
                            </select>
                          }
                          <div class="mirror-inline-hint">
                            {{ row.extraType === 'BONIFICATION_INCOME' ? messages.mirror.bonificationAttachHint : messages.mirror.sellAttachHint }}
                          </div>
                        } @else {
                          <span class="mirror-inline-hint">{{ messages.mirror.notApplicable }}</span>
                        }
                      </td>
                    }
                  </tr>
                  @if (mirrorRowErrors(row).length > 0) {
                    <tr class="draft-error-row">
                      <td [attr.colspan]="mirrorMode() === 'create' ? 5 : 5">{{ mirrorRowErrors(row).join(' · ') }}</td>
                    </tr>
                  }
                }
              </tbody>
            </table>
          </div>
          <div class="insert-actions">
            <button class="ghost-button" type="button" [disabled]="saving()" (click)="cancelMirrorModal()">
              {{ messages.mirror.back }}
            </button>
            <button class="primary-button" type="button" [disabled]="!canSubmitMirrorRows() || saving()" (click)="submitMirroredImport()">
              {{ saving() ? messages.mirror.submitting : messages.mirror.submit }}
            </button>
          </div>
        </section>
      </div>
    }
  `,
  styles: [`
    .modal-backdrop {
      align-items: center;
      background: rgba(15, 23, 42, 0.42);
      display: flex;
      inset: 0;
      justify-content: center;
      padding: 24px;
      position: fixed;
      z-index: 40;
    }

    .mirror-modal {
      background: color-mix(in srgb, var(--surface-strong) 96%, white 4%);
      border: 1px solid color-mix(in srgb, var(--border) 82%, rgba(15, 23, 42, 0.08));
      border-radius: 22px;
      box-shadow: 0 28px 72px rgba(15, 23, 42, 0.26);
      display: grid;
      gap: 16px;
      max-height: min(90vh, 960px);
      max-width: min(1040px, 100%);
      overflow: auto;
      padding: 20px;
      width: 100%;
    }

    .mirror-modal-header {
      align-items: start;
    }

    .page-warning {
      border-color: color-mix(in srgb, var(--warning, #b7791f) 35%, var(--border));
      display: grid;
      gap: 6px;
    }

    .page-warning p {
      color: var(--muted);
      margin: 0;
    }

    .investment-insert-col-date {
      width: 124px;
    }

    .field-hint {
      color: var(--muted);
      font-size: 0.9rem;
      margin: 0;
    }

    .insert-preview-state {
      margin-top: 12px;
      padding: 16px 18px;
    }

    .insert-table-wrap {
      background: transparent;
      border: 0;
      border-radius: inherit;
      box-shadow: none;
    }

    .insert-table-shell {
      min-height: 0;
    }

    .insert-preview-wrap {
      margin: 12px 0 0;
      max-width: none;
      width: 100%;
    }

    .insert-preview-asset-column {
      min-width: 240px;
      width: 32%;
    }

    .investment-insert-col-asset {
      width: 14%;
    }

    .investment-insert-col-type {
      width: 108px;
    }

    .investment-insert-col-account {
      width: 19%;
    }

    .investment-insert-actions-cell {
      padding-left: 14px;
    }

    .insert-preview-state strong,
    .insert-preview-state p {
      color: var(--warning, #b7791f);
    }

    .mirror-summary-cell {
      white-space: normal;
    }

    .mirror-summary-cell div + div {
      margin-top: 4px;
    }

    .mirror-table-attach {
      table-layout: auto;
      width: 100%;
    }

    .mirror-table-attach .mirror-transaction-cell {
      min-width: 340px;
      width: 36%;
    }

    .mirror-table-attach .mirror-pnl-cell {
      min-width: 280px;
      width: 28%;
    }

    .mirror-table-attach select {
      font-size: 0.88rem;
      min-width: 0;
      width: 100%;
    }

    .mirror-modal .insert-preview-table th,
    .mirror-modal .insert-preview-table td {
      padding: 8px 5px 8px 5px;
    }

    .mirror-modal .insert-preview-table .mirror-row-buy td {
      background: color-mix(in srgb, #6aa89c 14%, var(--surface));
    }

    .mirror-modal .insert-preview-table .mirror-row-sell td {
      background: color-mix(in srgb, #d6a56b 16%, var(--surface));
    }

    .mirror-modal .insert-preview-table .mirror-row-bonification td {
      background: color-mix(in srgb, #8aa0b8 14%, var(--surface));
    }
  `],
})
export class InvestmentInsertComponent implements OnInit, AfterViewInit {
  private readonly moneyVisibility = inject(MoneyVisibilityService);
  private readonly referenceData = inject(ReferenceDataService);
  private readonly userConfigService = inject(UserConfigService);
  private readonly destroyRef = inject(DestroyRef);
  readonly nav = uiMessages.investments.nav;
  readonly messages = uiMessages.investments.insert;
  readonly rows = signal<DraftOperationRow[]>([]);
  readonly saving = signal(false);
  readonly positionPreviewRows = signal<InvestmentPositionPreviewRow[]>([]);
  readonly previewBlocked = signal(false);
  readonly previewBlockedHint = signal<string | null>(null);
  readonly editingDateRowId = signal<number | null>(null);
  readonly mirrorModalOpen = signal(false);
  readonly mirrorMode = signal<MirrorMode>('create');
  readonly mirrorRows = signal<MirroredDraftRow[]>([]);
  readonly mirrorCandidates = signal<MirrorCandidate[]>([]);
  readonly mirrorPnlCandidates = signal<MirrorCandidate[]>([]);
  readonly mirrorPreviewRows = signal<InvestmentMirrorPreviewRow[]>([]);
  readonly activeAccounts = signal<Account[]>([]);
  readonly brokerageAccounts = signal<Account[]>([]);
  readonly investmentAccounts = signal<Account[]>([]);
  readonly sellAutomationConfigured = computed(() => {
    const integration = this.userConfigService.investmentIntegrationConfig();
    return !!integration.sell_gain_category_id && !!integration.sell_loss_category_id;
  });
  readonly bonificationAutomationConfigured = computed(() => {
    const integration = this.userConfigService.investmentIntegrationConfig();
    return !!integration.bonification_income_category_id;
  });

  private nextId = 1;
  private mirrorSourceApplied = false;
  private mirrorDestinationApplied = false;
  private readonly previewRowErrors = signal<Record<number, string[]>>({});
  private readonly previewRequests = new Subject<PreviewRequest>();

  constructor(
    private readonly investmentsService: InvestmentsService,
    private readonly transactionsService: TransactionsService,
    private readonly toast: ToastService,
    private readonly elementRef: ElementRef<HTMLElement>,
  ) {
    this.previewRequests
      .pipe(
        debounce((request) => timer(request.immediate ? 0 : 1000)),
        distinctUntilChanged((left, right) => left.signature === right.signature),
        switchMap((request) => {
          if (request.blocked) {
            return of<PreviewResult>({ position_preview_rows: [], clear: false, blocked: true });
          }
          if (!request.payload) {
            return of<PreviewResult>({ position_preview_rows: [], clear: true, blocked: false });
          }
          return this.investmentsService.previewImportOperations(request.payload).pipe(
            switchMap((preview) => of<PreviewResult>({ ...preview, clear: false, blocked: false })),
            catchError((error) => {
              return of(this.previewErrorResult(error));
            }),
          );
        }),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (preview) => {
          this.previewBlocked.set(Boolean(preview.blocked));
          this.previewBlockedHint.set(preview.message ?? null);
          this.previewRowErrors.set(preview.rowErrors ?? {});
          if (preview.blocked) {
            this.positionPreviewRows.set([]);
            this.mirrorPreviewRows.set([]);
            return;
          }
          if (preview.clear || preview.position_preview_rows.length > 0) {
            this.previewBlocked.set(false);
            this.previewBlockedHint.set(null);
            this.previewRowErrors.set({});
            this.positionPreviewRows.set(preview.position_preview_rows ?? []);
            this.mirrorPreviewRows.set(preview.mirror_preview_rows ?? []);
          }
        },
        error: () => {
          this.previewBlocked.set(false);
          this.previewBlockedHint.set(null);
          this.previewRowErrors.set({});
          this.positionPreviewRows.set([]);
          this.mirrorPreviewRows.set([]);
        },
      });

    effect(() => {
      const rows = this.rows();
      const request = untracked(() => this.buildPositionPreviewRequest(rows, false));
      this.previewRequests.next(request);
    });
  }

  ngOnInit(): void {
    this.resetRows();

    forkJoin({
      referenceData: this.referenceData.load(),
    }).subscribe({
      next: () => {
        this.activeAccounts.set(this.referenceData.accounts().filter((account) => !account.DeactivatedAt));
        this.brokerageAccounts.set(this.referenceData.accounts().filter((account) => !account.DeactivatedAt && account.asset_role === 'BROKERAGE'));
        this.investmentAccounts.set(this.referenceData.accounts().filter((account) => !account.DeactivatedAt && account.asset_role === 'INVESTMENT'));
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  ngAfterViewInit(): void {
    this.focusFirstCell();
  }

  addRows(count: number): void {
    this.rows.update((rows) => [...rows, ...Array.from({ length: count }, () => this.createEmptyRow())]);
  }

  addRowAfter(index: number): void {
    this.rows.update((rows) => {
      const next = [...rows];
      next.splice(index + 1, 0, this.createEmptyRow());
      return next;
    });
  }

  duplicateRow(index: number): void {
    const row = this.rows()[index];
    this.rows.update((rows) => {
      const next = [...rows];
      next.splice(index + 1, 0, { ...row, id: this.nextId++ });
      return next;
    });
  }

  clearRow(row: DraftOperationRow): void {
    Object.assign(row, this.createEmptyRow(row.id));
    this.rows.update((rows) => [...rows]);
  }

  clearEmptyRows(): void {
    const nonEmptyRows = this.rows().filter((row) => !this.isEmpty(row));
    this.rows.set(nonEmptyRows.length > 0 ? nonEmptyRows : [this.createEmptyRow()]);
  }

  onDateInput(row: DraftOperationRow, value: string): void {
    this.updateRow(row.id, { date: formatDraftDateInput(value) });
  }

  startDateEdit(row: DraftOperationRow, input?: HTMLInputElement): void {
    this.editingDateRowId.set(row.id);
    if (!row.date.trim() || !input) {
      return;
    }
    setTimeout(() => {
      if (this.editingDateRowId() === row.id) {
        input.select();
        input.setSelectionRange(0, input.value.length);
      }
    }, 0);
  }

  finishDateEdit(row: DraftOperationRow): void {
    this.updateRow(row.id, { date: normalizeDraftDate(row.date) });
    this.editingDateRowId.set(null);
  }

  dateInputValue(row: DraftOperationRow): string {
    return row.date;
  }

  normalizeDate(row: DraftOperationRow): void {
    this.updateRow(row.id, { date: normalizeDraftDate(row.date) });
  }

  normalizeAsset(row: DraftOperationRow): void {
    this.updateRow(row.id, { assetCode: row.assetCode.trim().toUpperCase() });
  }

  normalizeQuantity(row: DraftOperationRow): void {
    this.updateRow(row.id, { quantity: row.quantity.replace(/[^\d]/g, '') });
  }

  onMoneyInput(row: DraftOperationRow, field: 'unitPrice' | 'totalFeeAmount', value: string): void {
    const manualFlag = this.moneyManualFlag(field);
    if (!value.trim()) {
      this.updateRow(row.id, {
        [field]: '',
        [manualFlag]: false,
      } as Pick<DraftOperationRow, typeof field | typeof manualFlag>);
      return;
    }

    if (row[manualFlag]) {
      const next: Pick<DraftOperationRow, typeof field | typeof manualFlag> = {
        [field]: value,
        [manualFlag]: row[manualFlag],
      } as Pick<DraftOperationRow, typeof field | typeof manualFlag>;
      if (!value.includes(',')) {
        next[manualFlag] = false;
      }
      this.updateRow(row.id, next);
      return;
    }

    this.updateRow(row.id, { [field]: formatAmountDigitsAsCents(value) } as Pick<DraftOperationRow, typeof field>);
  }

  finishMoneyEdit(row: DraftOperationRow, field: 'unitPrice' | 'totalFeeAmount'): void {
    const manualFlag = this.moneyManualFlag(field);
    const normalized = normalizeDraftAmount(row[field]);
    this.updateRow(row.id, {
      [field]: normalized,
      [manualFlag]: normalized.includes(','),
    } as Pick<DraftOperationRow, typeof field | typeof manualFlag>);
  }

  normalizeMoney(row: DraftOperationRow, field: 'unitPrice' | 'totalFeeAmount'): void {
    if (!row[field].trim()) {
      const manualFlag = this.moneyManualFlag(field);
      this.updateRow(row.id, {
        [field]: '',
        [manualFlag]: false,
      } as Pick<DraftOperationRow, typeof field | typeof manualFlag>);
      return;
    }
    const manualFlag = this.moneyManualFlag(field);
    const normalized = normalizeDraftAmount(row[field]);
    this.updateRow(row.id, {
      [field]: normalized,
      [manualFlag]: normalized.includes(','),
    } as Pick<DraftOperationRow, typeof field | typeof manualFlag>);
  }

  updateAssetCode(row: DraftOperationRow, value: string): void {
    this.updateRow(row.id, { assetCode: value });
  }

  updateQuantity(row: DraftOperationRow, value: string): void {
    this.updateRow(row.id, { quantity: value });
  }

  onOperationTypeChange(row: DraftOperationRow, value: InvestmentOperationType | ''): void {
    this.updateRow(row.id, { operationType: value });
    this.requestPreviewNow();
  }

  onBrokerageAccountCodeChange(row: DraftOperationRow, value: string): void {
    this.updateRow(row.id, { brokerageAccountCode: value });
    this.requestPreviewNow();
  }

  onInvestmentAccountCodeChange(row: DraftOperationRow, value: string): void {
    this.updateRow(row.id, { investmentAccountCode: value });
    this.requestPreviewNow();
  }

  handlePaste(event: ClipboardEvent, startRowIndex: number, startColumnIndex: number): void {
    const text = event.clipboardData?.getData('text/plain') ?? '';
    if (!text) {
      return;
    }
    event.preventDefault();
    const pastedRows = text
      .replace(/\r/g, '')
      .split('\n')
      .filter((line) => line.length > 0)
      .map((line) => {
        const values = line.split('\t');
        if (startColumnIndex == 0 && values.length === INSERT_COLUMN_COUNT - 1) {
          values.splice(4, 0, '');
        }
        return values;
      });
    this.ensureRowCount(startRowIndex + pastedRows.length);
    const rows = this.rows();
    pastedRows.forEach((values, rowOffset) => {
      const row = rows[startRowIndex + rowOffset];
      if (!row) {
        return;
      }
      values.slice(0, INSERT_COLUMN_COUNT - startColumnIndex).forEach((value, columnOffset) => {
        this.applyPastedValue(row, startColumnIndex + columnOffset, value);
      });
      this.normalizeDate(row);
      this.normalizeAsset(row);
      this.normalizeQuantity(row);
      this.normalizeMoney(row, 'unitPrice');
      this.normalizeMoney(row, 'totalFeeAmount');
    });
  }

  moveCell(event: KeyboardEvent): void {
    if (event.defaultPrevented) {
      return;
    }
    if (event.key !== 'Enter' && event.key !== 'Tab') {
      return;
    }

    event.preventDefault();

    if (event.key === 'Enter') {
      this.focusFirstCellInAdjacentRow(event.target, event.shiftKey ? -1 : 1);
      return;
    }

    const cells = Array.from(
      document.querySelectorAll<HTMLElement>('[data-grid-cell]:not([disabled])'),
    );
    const current = event.target as HTMLElement;
    const currentIndex = cells.indexOf(current);
    if (currentIndex === -1) {
      return;
    }
    const delta = event.shiftKey ? -1 : 1;
    const next = cells[currentIndex + delta];
    next?.focus();
  }

  rowValidation(row: DraftOperationRow): RowValidation {
    const validation = this.localRowValidation(row);
    validation.errors.push(...(this.previewRowErrors()[row.id] ?? []));
    validation.valid = validation.errors.length === 0;
    return validation;
  }

  private localRowValidation(row: DraftOperationRow): RowValidation {
    if (this.isEmpty(row)) {
      return { valid: true, errors: [] };
    }

    const errors: string[] = [];
    const dateKey = normalizedDateKey(row.date);
    if (!dateKey) {
      errors.push('Data inválida');
    }
    if (!row.assetCode.trim()) {
      errors.push('Ticker obrigatório');
    }
    if (!row.operationType) {
      errors.push('Operação obrigatória');
    }
    if (!row.brokerageAccountCode.trim()) {
      errors.push('Conta corretora obrigatória');
    }
    if (!row.investmentAccountCode.trim()) {
      errors.push('Conta de investimento obrigatória');
    }
    const quantity = Number(row.quantity);
    if (!row.quantity || !Number.isInteger(quantity) || quantity <= 0) {
      errors.push('Quantidade inválida');
    }
    if (decimalToCents(row.unitPrice) < 0) {
      errors.push('Preço unitário inválido');
    }
    if (decimalToCents(row.totalFeeAmount) < 0) {
      errors.push('Taxa total inválida');
    }
    if (row.operationType === 'AMORTIZATION' && decimalToCents(row.totalFeeAmount) !== 0) {
      errors.push(this.messages.validation.amortizationFeeMustBeZero);
    }
    const feeConflict = this.sameDayBrokerageFeeConflict(row);
    if (feeConflict) {
      errors.push(feeConflict);
    }

    return { valid: errors.length === 0, errors };
  }

  canSubmit(): boolean {
    const filled = this.filledRows();
    return filled.length > 0 && filled.every((row) => this.rowValidation(row).valid);
  }

  submit(): void {
    if (!this.canSubmit()) {
      return;
    }
    const filled = this.filledRows();
    const mirroredCandidates = filled.filter((row) =>
      row.operationType === 'BUY'
      || row.operationType === 'SELL'
      || row.operationType === 'BONIFICATION'
      || row.operationType === 'AMORTIZATION',
    );
    if (mirroredCandidates.length > 0 && window.confirm(this.messages.mirror.confirm)) {
      if (!this.sellAutomationConfigured() && mirroredCandidates.some((row) => row.operationType === 'SELL')) {
        this.toast.error(this.messages.mirror.sellConfigRequired);
        return;
      }
      if (!this.bonificationAutomationConfigured() && mirroredCandidates.some((row) => row.operationType === 'BONIFICATION')) {
        this.toast.error(this.messages.mirror.bonificationConfigRequired);
        return;
      }
      this.openMirrorModal();
      return;
    }
    this.submitImport(false, []);
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrency(value);
  }

  assetLabel(code: string, name: string): string {
    return investmentAssetLabel(code, name);
  }

  mirrorCandidateGroups(row: MirroredDraftRow): MirrorCandidateGroup[] {
    const brokerageCode = row.brokerageAccountCode.trim().toLowerCase();
    const filtered = this.mirrorCandidates().filter((candidate) =>
      !brokerageCode
      || candidate.accountCode.trim().toLowerCase() === brokerageCode
      || candidate.transferAccountCode.trim().toLowerCase() === brokerageCode,
    );
    const allTransfers = [...filtered].sort((left, right) => right.date.localeCompare(left.date));
    const nearbyTransfers = filtered
      .filter((candidate) => daysBetweenIsoDates(candidate.date, row.date) <= 7)
      .sort((left, right) => {
        const leftDistance = daysBetweenIsoDates(left.date, row.date);
        const rightDistance = daysBetweenIsoDates(right.date, row.date);
        if (leftDistance === rightDistance) {
          return right.date.localeCompare(left.date);
        }
        return leftDistance - rightDistance;
      });

    const groups: MirrorCandidateGroup[] = [];
    if (nearbyTransfers.length > 0) {
      groups.push({ label: 'Transferências perto da data', items: nearbyTransfers });
    }
    groups.push({ label: 'Todas as transferências', items: allTransfers });
    return groups;
  }

  mirrorBrokerageHint(row: MirroredDraftRow): string {
    if (!row.brokerageAccountCode) {
      return '';
    }
    return `Filtrando pela conta corretora: ${this.referenceData.accountName(row.brokerageAccountCode)}`;
  }

  mirrorSummaryLines(row: MirroredDraftRow): string[] {
    const lines = [`${this.messages.mirror.transferAmount}: ${this.money(row.transferAmount)}`];
    if (row.extraType === 'REALIZED_PNL') {
      lines.push(`${this.mirrorRealizedPnlLabel(row.extraAmount)}: ${this.money(Math.abs(row.extraAmount))}`);
      lines.push(`${this.messages.mirror.saleNetAmount}: ${this.money(row.transferAmount + row.extraAmount)}`);
    } else if (row.extraType === 'BONIFICATION_INCOME') {
      lines.push(`${this.messages.mirror.bonificationIncomeAmount}: ${this.money(row.extraAmount)}`);
    }
    return lines;
  }

  private mirrorRealizedPnlLabel(amount: number): string {
    return amount < 0 ? 'Prejuízo' : 'Lucro';
  }

  mirrorDisplayDate(value: string): string {
    return toCompactMirrorOptionDate(value);
  }

  private expectedMirrorExtraCategoryCode(row: MirroredDraftRow): string | null {
    const integration = this.userConfigService.investmentIntegrationConfig();
    if (row.extraType === 'BONIFICATION_INCOME') {
      return this.categoryCodeById(integration.bonification_income_category_id ?? null);
    }
    if (row.extraType === 'REALIZED_PNL') {
      return this.categoryCodeById(
        row.extraAmount >= 0
          ? (integration.sell_gain_category_id ?? null)
          : (integration.sell_loss_category_id ?? null),
      );
    }
    return null;
  }

  private categoryCodeById(categoryId: string | null): string | null {
    if (!categoryId) {
      return null;
    }
    return this.referenceData.flatCategories().find((category) => category.ID === categoryId)?.Code ?? null;
  }

  mirrorPnlCandidateGroups(row: MirroredDraftRow): MirrorCandidateGroup[] {
    const brokerageCode = row.brokerageAccountCode.trim().toLowerCase();
    const expectsPositivePnl = row.extraAmount >= 0;
    const expectedCategoryCode = this.expectedMirrorExtraCategoryCode(row);
    const filtered = this.mirrorPnlCandidates().filter((candidate) => {
      if (brokerageCode && candidate.accountCode.trim().toLowerCase() !== brokerageCode) {
        return false;
      }
      if (expectedCategoryCode && candidate.categoryCode !== expectedCategoryCode) {
        return false;
      }
      if (row.extraType === 'BONIFICATION_INCOME') {
        return candidate.amount >= 0;
      }
      return expectsPositivePnl ? candidate.amount >= 0 : candidate.amount < 0;
    });
    const allCandidates = [...filtered].sort((left, right) => right.date.localeCompare(left.date));
    const nearbyCandidates = filtered
      .filter((candidate) => daysBetweenIsoDates(candidate.date, row.date) <= 7)
      .sort((left, right) => {
        const leftDistance = daysBetweenIsoDates(left.date, row.date);
        const rightDistance = daysBetweenIsoDates(right.date, row.date);
        if (leftDistance === rightDistance) {
          return right.date.localeCompare(left.date);
        }
        return leftDistance - rightDistance;
      });

    const groups: MirrorCandidateGroup[] = [];
    const nearbyLabel = row.extraType === 'BONIFICATION_INCOME' ? 'Receitas perto da data' : 'Resultados perto da data';
    const allLabel = row.extraType === 'BONIFICATION_INCOME' ? 'Todas as receitas' : 'Todos os resultados';
    if (nearbyCandidates.length > 0) {
      groups.push({ label: nearbyLabel, items: nearbyCandidates });
    }
    groups.push({ label: allLabel, items: allCandidates });
    return groups;
  }

  private filledRows(): DraftOperationRow[] {
    return this.rows().filter((row) => !this.isEmpty(row));
  }

  mirrorRowErrors(row: MirroredDraftRow): string[] {
    const errors: string[] = [];
    if (this.mirrorMode() === 'attach') {
      if (!row.transactionId) {
        errors.push('Transferência obrigatória');
      }
      if (row.extraType !== 'NONE' && row.attachExtraTransaction && !row.extraTransactionId) {
        errors.push(row.extraType === 'BONIFICATION_INCOME' ? 'Receita obrigatória' : 'Resultado obrigatório');
      }
      return errors;
    }
    if (!row.sourceAccountCode) {
      errors.push('Conta origem obrigatória');
    }
    if (!row.destinationAccountCode) {
      errors.push('Conta destino obrigatória');
    }
    if (row.sourceAccountCode && row.destinationAccountCode && row.sourceAccountCode === row.destinationAccountCode) {
      errors.push('Contas precisam ser diferentes');
    }
    return errors;
  }

  canSubmitMirrorRows(): boolean {
    return this.mirrorRows().length > 0 && this.mirrorRows().every((row) => this.mirrorRowErrors(row).length === 0);
  }

  cancelMirrorModal(): void {
    if (this.saving()) {
      return;
    }
    this.mirrorModalOpen.set(false);
    this.mirrorRows.set([]);
    this.mirrorCandidates.set([]);
    this.mirrorPnlCandidates.set([]);
  }

  updateMirrorAccount(row: MirroredDraftRow, field: 'sourceAccountCode' | 'destinationAccountCode', value: string): void {
    const normalized = value.trim();
    row[field] = normalized;

    if (normalized) {
      if (field === 'sourceAccountCode' && !this.mirrorSourceApplied) {
        this.mirrorSourceApplied = true;
        this.mirrorRows.update((rows) => rows.map((candidate) => ({ ...candidate, sourceAccountCode: normalized })));
        return;
      }
      if (field === 'destinationAccountCode' && !this.mirrorDestinationApplied) {
        this.mirrorDestinationApplied = true;
        this.mirrorRows.update((rows) => rows.map((candidate) => ({ ...candidate, destinationAccountCode: normalized })));
        return;
      }
    }

    this.mirrorRows.update((rows) => [...rows]);
  }

  updateMirrorTransaction(row: MirroredDraftRow, value: string): void {
    row.transactionId = value.trim();
    this.mirrorRows.update((rows) => [...rows]);
  }

  toggleMirrorExtraTransaction(row: MirroredDraftRow, value: boolean): void {
    row.attachExtraTransaction = Boolean(value);
    if (!row.attachExtraTransaction) {
      row.extraTransactionId = '';
    }
    this.mirrorRows.update((rows) => [...rows]);
  }

  updateMirrorExtraTransaction(row: MirroredDraftRow, value: string): void {
    row.extraTransactionId = value.trim();
    this.mirrorRows.update((rows) => [...rows]);
  }

  submitMirroredImport(): void {
    if (!this.canSubmitMirrorRows()) {
      return;
    }
    if (this.mirrorMode() === 'attach') {
      const mismatches = this.mirrorRows()
        .map((row) => {
          const candidate = this.mirrorCandidates().find((item) => item.id === row.transactionId);
          if (!candidate || candidate.amount === row.transferAmount) {
            return null;
          }
          return `${row.description}: ${this.money(candidate.amount)} -> ${this.money(row.transferAmount)}`;
        })
        .filter((item): item is string => Boolean(item));
      if (mismatches.length > 0) {
        const shouldContinue = window.confirm(
          `As transferências selecionadas terão seus valores ajustados para seguir as operações:\n\n${mismatches.join('\n')}\n\nDeseja continuar?`,
        );
        if (!shouldContinue) {
          return;
        }
      }
    }
    this.submitImport(true, this.mirrorRows());
  }

  private focusFirstCell(): void {
    setTimeout(() => {
      const input = this.elementRef.nativeElement.querySelector<HTMLElement>(
        'tbody tr:first-child [data-grid-cell]',
      );
      input?.focus();
    }, 0);
  }

  private resetRows(): void {
    this.rows.set(Array.from({ length: INITIAL_ROWS }, () => this.createEmptyRow()));
  }

  private openMirrorModal(): void {
    this.mirrorSourceApplied = false;
    this.mirrorDestinationApplied = false;
    this.mirrorMode.set('create');
    this.mirrorRows.set(this.mirrorPreviewRows().map((row) => ({
      clientRowId: row.client_row_id,
      groupKey: row.group_key,
      operationType: row.operation_type,
      brokerageAccountCode: row.brokerage_account_code,
      investmentAccountCode: row.investment_account_code,
      date: row.date,
      description: row.description,
      transferAmount: row.transfer_amount,
      extraAmount: row.extra_amount,
      extraType: row.extra_type,
      sourceAccountCode: row.source_account_code,
      destinationAccountCode: row.destination_account_code,
      transactionId: '',
      attachExtraTransaction: false,
      extraTransactionId: '',
    })));
    this.mirrorCandidates.set([]);
    this.mirrorPnlCandidates.set([]);
    this.mirrorModalOpen.set(true);

    const brokerageAccountCodes = Array.from(new Set(this.mirrorRows().map((row) => row.brokerageAccountCode.trim()).filter((code) => code.length > 0)));
    forkJoin({
      transfers: this.transactionsService.list({
        operation: 'transfer',
        account_code: brokerageAccountCodes,
        limit: 1000,
      }),
      pnl: this.transactionsService.list({
        operation: ['credit', 'debit'],
        account_code: brokerageAccountCodes,
        limit: 1000,
      }),
    }).subscribe({
      next: ({ transfers, pnl }) => {
        this.mirrorCandidates.set(
          (transfers.transactions ?? [])
            .filter((tx) => Boolean(tx.transfer_id) && tx.account_transfer && !tx.is_investment_operation_mirror)
            .map((tx) => ({
              id: tx.id,
              amount: Math.abs(tx.amount),
              date: tx.date,
              accountCode: tx.account_code,
              transferAccountCode: tx.account_transfer ?? '',
              label: buildCompactMirrorOptionLabel(tx.date, tx.description, tx.amount),
            })),
        );
        this.mirrorPnlCandidates.set(
          (pnl.transactions ?? [])
            .filter((tx) => !tx.transfer_id && !tx.is_investment_operation_mirror)
            .map((tx) => ({
              id: tx.id,
              amount: tx.amount,
              date: tx.date,
              accountCode: tx.account_code,
              transferAccountCode: '',
              categoryCode: tx.category_code,
              label: buildCompactMirrorOptionLabel(tx.date, tx.description, tx.amount),
            })),
        );
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  private submitImport(createMirroredTransactions: boolean, mirroredRows: MirroredDraftRow[]): void {
    const filled = this.filledRows();
    this.saving.set(true);
    this.investmentsService.importOperations({
      operations: filled.map((row) => ({
        client_row_id: this.clientRowId(row),
        asset_code: row.assetCode,
        brokerage_account_code: row.brokerageAccountCode,
        investment_account_code: row.investmentAccountCode,
        operation_type: row.operationType as InvestmentOperationType,
        date: dateInputToIso(brazilianDateToQuery(row.date)),
        quantity: Number(row.quantity),
        unit_price: decimalToCents(row.unitPrice),
        total_fee_amount: decimalToCents(row.totalFeeAmount),
        notes: '',
      })),
      create_mirrored_transactions: createMirroredTransactions,
      mirrored_transactions: mirroredRows.map((row) => ({
        client_row_id: row.clientRowId,
        source_account_code: row.sourceAccountCode,
        destination_account_code: row.destinationAccountCode,
        transaction_id: row.transactionId || null,
        realized_pnl_transaction_id:
          row.extraType === 'REALIZED_PNL' && row.attachExtraTransaction ? (row.extraTransactionId || null) : null,
        bonification_income_transaction_id:
          row.extraType === 'BONIFICATION_INCOME' && row.attachExtraTransaction ? (row.extraTransactionId || null) : null,
      })),
    }).subscribe({
      next: (result) => {
        this.toast.success(
          result.mirroring_enabled
            ? 'Operações e transferências vinculadas salvas.'
            : 'Operações salvas.',
        );
        this.mirrorModalOpen.set(false);
        this.mirrorRows.set([]);
        this.mirrorPnlCandidates.set([]);
        this.resetRows();
        forkJoin({
          referenceData: this.referenceData.reload(),
        }).subscribe({
          next: () => {
            this.activeAccounts.set(this.referenceData.accounts().filter((account) => !account.DeactivatedAt));
            this.brokerageAccounts.set(this.referenceData.accounts().filter((account) => !account.DeactivatedAt && account.asset_role === 'BROKERAGE'));
            this.investmentAccounts.set(this.referenceData.accounts().filter((account) => !account.DeactivatedAt && account.asset_role === 'INVESTMENT'));
          },
          error: (error) => this.toast.error(getApiErrorMessage(error)),
        });
        this.saving.set(false);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.saving.set(false);
      },
    });
  }

  private createEmptyRow(id = this.nextId++): DraftOperationRow {
    return {
      id,
      date: '',
      assetCode: '',
      operationType: '',
      brokerageAccountCode: '',
      investmentAccountCode: '',
      quantity: '',
      unitPrice: '',
      unitPriceManualDecimal: false,
      totalFeeAmount: '',
      totalFeeAmountManualDecimal: false,
    };
  }

  private updateRow(rowID: number, patch: Partial<DraftOperationRow>): void {
    let changed = false;
    this.rows.update((rows) => rows.map((row) => {
      if (row.id !== rowID) {
        return row;
      }

      for (const [key, value] of Object.entries(patch)) {
        if (row[key as keyof DraftOperationRow] !== value) {
          changed = true;
          break;
        }
      }

      return changed ? { ...row, ...patch } : row;
    }));

    if (changed) {
      this.resetPreviewValidationFeedback();
    }
  }

  private isEmpty(row: DraftOperationRow): boolean {
    return !row.date && !row.assetCode && !row.operationType && !row.brokerageAccountCode && !row.investmentAccountCode && !row.quantity && !row.unitPrice;
  }

  private ensureRowCount(requiredCount: number): void {
    const missing = requiredCount - this.rows().length;
    if (missing <= 0) {
      return;
    }
    this.addRows(missing);
  }

  private applyPastedValue(row: DraftOperationRow, columnIndex: number, rawValue: string): void {
    const value = rawValue.trim();
    switch (columnIndex) {
      case 0:
        row.date = value;
        break;
      case 1:
        row.assetCode = value;
        break;
      case 2:
        row.operationType = normalizeOperationType(value);
        break;
      case 3:
        row.brokerageAccountCode = this.resolveBrokerageAccountPasteValue(value);
        break;
      case 4:
        row.investmentAccountCode = this.resolveInvestmentAccountPasteValue(value);
        break;
      case 5:
        row.quantity = value;
        break;
      case 6:
        row.unitPrice = value;
        row.unitPriceManualDecimal = value.includes(',');
        break;
      case 7:
        row.totalFeeAmount = value;
        row.totalFeeAmountManualDecimal = value.includes(',');
        break;
    }
  }

  private moneyManualFlag(field: 'unitPrice' | 'totalFeeAmount'): 'unitPriceManualDecimal' | 'totalFeeAmountManualDecimal' {
    return field === 'unitPrice' ? 'unitPriceManualDecimal' : 'totalFeeAmountManualDecimal';
  }

  private resolveBrokerageAccountPasteValue(value: string): string {
    const normalized = value.trim();
    if (!normalized) {
      return '';
    }

    const byCode = this.brokerageAccounts().find(
      (account) => account.Code.toLocaleLowerCase('pt-BR') === normalized.toLocaleLowerCase('pt-BR'),
    );
    if (byCode) {
      return byCode.Code;
    }

    const byName = this.brokerageAccounts().find(
      (account) => account.Name.trim().toLocaleLowerCase('pt-BR') === normalized.toLocaleLowerCase('pt-BR'),
    );
    return byName?.Code ?? normalized;
  }

  private resolveInvestmentAccountPasteValue(value: string): string {
    const normalized = value.trim();
    if (!normalized) {
      return '';
    }

    const byCode = this.investmentAccounts().find(
      (account) => account.Code.toLocaleLowerCase('pt-BR') === normalized.toLocaleLowerCase('pt-BR'),
    );
    if (byCode) {
      return byCode.Code;
    }

    const byName = this.investmentAccounts().find(
      (account) => account.Name.trim().toLocaleLowerCase('pt-BR') === normalized.toLocaleLowerCase('pt-BR'),
    );
    return byName?.Code ?? normalized;
  }

  private focusFirstCellInAdjacentRow(target: EventTarget | null, delta: -1 | 1): void {
    const currentCell = target instanceof HTMLElement ? target : null;
    const currentRow = currentCell?.closest('tr');
    let siblingRow = currentRow?.[delta > 0 ? 'nextElementSibling' : 'previousElementSibling'];

    while (siblingRow instanceof HTMLTableRowElement) {
      const firstCell = siblingRow.querySelector<HTMLElement>('[data-grid-cell]:not([disabled])');
      if (firstCell) {
        firstCell.focus();
        return;
      }
      siblingRow = siblingRow[delta > 0 ? 'nextElementSibling' : 'previousElementSibling'];
    }
  }

  private clientRowId(row: DraftOperationRow): string {
    return mirrorDraftRowId(row.id);
  }

  requestPreviewNow(): void {
    this.previewRequests.next(this.buildPositionPreviewRequest(this.rows(), true));
  }

  private buildPositionPreviewRequest(rows: DraftOperationRow[], immediate: boolean): PreviewRequest {
    const filledRows = rows.filter((row) => !this.isEmpty(row));
    if (filledRows.length === 0) {
      return { signature: 'empty', payload: null, immediate, blocked: false };
    }

    const invalidRows = filledRows.filter((row) => !this.localRowValidation(row).valid);
    if (invalidRows.length > 0) {
      return {
        signature: this.previewBlockedSignature(filledRows),
        payload: null,
        immediate,
        blocked: true,
      };
    }

    const operations = filledRows.map((row) => ({
      client_row_id: this.clientRowId(row),
      asset_code: row.assetCode.trim().toUpperCase(),
      brokerage_account_code: row.brokerageAccountCode.trim(),
      investment_account_code: row.investmentAccountCode.trim(),
      operation_type: row.operationType as InvestmentOperationType,
      date: dateInputToIso(brazilianDateToQuery(row.date)),
      quantity: Number(row.quantity),
      unit_price: decimalToCents(row.unitPrice),
      total_fee_amount: decimalToCents(row.totalFeeAmount),
      notes: '',
    }));

    return {
      signature: JSON.stringify(operations),
      payload: {
        operations,
        create_mirrored_transactions: false,
        mirrored_transactions: [],
      },
      immediate,
      blocked: false,
    };
  }

  private sameDayBrokerageFeeConflict(row: DraftOperationRow): string {
    if (this.isEmpty(row)) {
      return '';
    }
    const dateKey = normalizedDateKey(row.date);
    const brokerageCode = row.brokerageAccountCode.trim().toLocaleLowerCase('pt-BR');
    if (!dateKey || !brokerageCode) {
      return '';
    }

    const groupRows = this.rows().filter((candidate) =>
      !this.isEmpty(candidate)
      && normalizedDateKey(candidate.date) === dateKey
      && candidate.brokerageAccountCode.trim().toLocaleLowerCase('pt-BR') === brokerageCode,
    );
    const distinctFeeValues = Array.from(
      new Set(groupRows.map((candidate) => decimalToCents(candidate.totalFeeAmount))),
    );
    return distinctFeeValues.length > 1 ? this.messages.states.sameDayBrokerageFeeConflict : '';
  }

  private previewBlockedSignature(rows: DraftOperationRow[]): string {
    return JSON.stringify(rows.map((row) => ({
      id: row.id,
      date: row.date,
      brokerage: row.brokerageAccountCode,
      fee: row.totalFeeAmount,
      asset: row.assetCode,
      quantity: row.quantity,
      type: row.operationType,
      investment: row.investmentAccountCode,
      unitPrice: row.unitPrice,
    })));
  }

  private previewErrorResult(error: unknown): PreviewResult {
    const rowErrors = this.previewRowErrorsFromApiError(error);
    return {
      position_preview_rows: [],
      clear: false,
      blocked: true,
      rowErrors,
      message: Object.keys(rowErrors).length > 0 ? null : getApiErrorMessage(error),
    };
  }

  private previewRowErrorsFromApiError(error: unknown): Record<number, string[]> {
    if (getApiErrorCode(error) !== 'investment.operation.sell.exceeds.position') {
      return {};
    }

    const details = getApiErrorDetails(error);
    const clientRowIDValue = details?.['client_row_id'];
    const clientRowID = typeof clientRowIDValue === 'string' ? clientRowIDValue.trim() : '';
    if (!clientRowID) {
      return {};
    }

    const row = this.rows().find((candidate) => this.clientRowId(candidate) === clientRowID);
    if (!row) {
      return {};
    }

    const attemptedQuantity = this.integerDetailValue(details?.['attempted_quantity']);
    const availableQuantity = this.integerDetailValue(details?.['available_quantity']);

    return {
      [row.id]: [this.sellExceedsPositionMessage(attemptedQuantity, availableQuantity)],
    };
  }

  private resetPreviewValidationFeedback(): void {
    this.previewBlockedHint.set(null);
    this.previewRowErrors.set({});
  }

  private sellExceedsPositionMessage(attemptedQuantity: number | null, availableQuantity: number | null): string {
    if (attemptedQuantity == null || availableQuantity == null) {
      return this.messages.states.sellExceedsPosition
        .replace('{attempted}', '?')
        .replace('{available}', '?');
    }

    return this.messages.states.sellExceedsPosition
      .replace('{attempted}', this.integerQuantity(attemptedQuantity))
      .replace('{available}', this.integerQuantity(availableQuantity));
  }

  private integerDetailValue(value: unknown): number | null {
    return typeof value === 'number' && Number.isInteger(value) ? value : null;
  }

  private integerQuantity(value: number): string {
    return value.toLocaleString('pt-BR');
  }
}

function normalizeOperationType(value: string): InvestmentOperationType | '' {
  const normalized = value.trim().toLowerCase();
  if (normalized === 'buy' || normalized === 'compra') {
    return 'BUY';
  }
  if (normalized === 'sell' || normalized === 'venda') {
    return 'SELL';
  }
  if (normalized === 'bonification' || normalized === 'bonificação' || normalized === 'bonificacao') {
    return 'BONIFICATION';
  }
  if (normalized === 'amortization' || normalized === 'amortização' || normalized === 'amortizacao') {
    return 'AMORTIZATION';
  }
  return '';
}

function formatDraftDateInput(value: string): string {
  const digits = value.replace(/\D/g, '').slice(0, 8);
  if (digits.length <= 2) {
    return digits;
  }
  if (digits.length <= 4) {
    return `${digits.slice(0, 2)}/${digits.slice(2)}`;
  }
  return `${digits.slice(0, 2)}/${digits.slice(2, 4)}/${digits.slice(4)}`;
}

function normalizeDraftDate(value: string): string {
  const normalized = value.trim();
  if (!normalized) {
    return '';
  }

  const shortMatch = /^(\d{1,2})$/.exec(normalized);
  if (shortMatch) {
    const day = Number(shortMatch[1]);
    if (day < 1 || day > 31) {
      return normalized;
    }

    const today = new Date();
    let year = today.getFullYear();
    let month = today.getMonth() + 1;
    if (day > today.getDate()) {
      month -= 1;
      if (month === 0) {
        month = 12;
        year -= 1;
      }
    }

    return `${String(day).padStart(2, '0')}/${String(month).padStart(2, '0')}/${year}`;
  }

  const fullMatch = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(normalized);
  if (!fullMatch) {
    return normalized;
  }

  const [, day, month, year] = fullMatch;
  return `${day.padStart(2, '0')}/${month.padStart(2, '0')}/${year}`;
}

function normalizedDateKey(value: string): string | null {
  const normalized = normalizeDraftDate(value);
  return /^\d{2}\/\d{2}\/\d{4}$/.test(normalized) ? normalized : null;
}

function normalizeDraftAmount(value: string): string {
  const normalized = value.trim();
  if (!normalized) {
    return '';
  }

  if (/^\d+(?:\.\d{3})*(,\d{2})$/.test(normalized)) {
    return normalized;
  }

  if (/^\d+(?:\.\d{3})*$/.test(normalized)) {
    return `${normalized},00`;
  }

  if (/^\d+(?:\.\d{3})*,\d$/.test(normalized)) {
    return `${normalized}0`;
  }

  return normalized;
}

function formatAmountDigitsAsCents(value: string): string {
  const digits = value.replace(/\D/g, '');
  if (!digits) {
    return '';
  }

  const integer = digits.slice(0, -2) || '0';
  const cents = digits.slice(-2).padStart(2, '0');
  return `${stripLeadingZeros(integer)},${cents}`;
}

function stripLeadingZeros(value: string): string {
  const normalized = value.replace(/^0+(?=\d)/, '');
  return normalized === '' ? '0' : normalized;
}

function buildCompactMirrorOptionLabel(date: string, description: string, amount: number): string {
  const compactDescription = truncateMirrorOptionDescription(description, MIRROR_OPTION_DESCRIPTION_MAX_CHARS);
  return `${toCompactMirrorOptionDate(date)}${MIRROR_OPTION_SEPARATOR}${compactDescription}${MIRROR_OPTION_SEPARATOR}${toCompactMirrorOptionAmount(amount)}`;
}

function truncateMirrorOptionDescription(description: string, maxChars: number): string {
  const normalized = description.trim().replace(/\s+/g, ' ');
  if (normalized.length <= maxChars) {
    return normalized;
  }
  if (maxChars <= 3) {
    return normalized.slice(0, maxChars);
  }
  return `${normalized.slice(0, maxChars - 3)}...`;
}

function toCompactMirrorOptionDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const isoDate = date.toISOString().slice(0, 10);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(isoDate)) {
    return value;
  }
  return `${isoDate.slice(8, 10)}/${isoDate.slice(5, 7)}/${isoDate.slice(2, 4)}`;
}

function toCompactMirrorOptionAmount(amount: number): string {
  return centsToCurrency(Math.abs(amount)).replace(/\u00a0/g, ' ');
}

function mirrorDraftRowId(rowId: number): string {
  return `row-${rowId}`;
}

function daysBetweenIsoDates(left: string, right: string): number {
  const leftTime = Date.parse(left);
  const rightTime = Date.parse(right);
  if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) {
    return Number.MAX_SAFE_INTEGER;
  }
  return Math.abs(Math.round((leftTime - rightTime) / (24 * 60 * 60 * 1000)));
}
