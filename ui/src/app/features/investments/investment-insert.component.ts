import { AfterViewInit, Component, ElementRef, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { forkJoin } from 'rxjs';

import { InvestmentsService } from '../../data/investments.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { uiMessages } from '../../shared/messages';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { brazilianDateToQuery, centsToDecimal, dateInputToIso, decimalToCents } from '../../shared/money';
import { InvestmentAsset, InvestmentOperationType, InvestmentPosition } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';
import { investmentAssetLabel } from '../../shared/labels';

interface DraftOperationRow {
  id: number;
  date: string;
  assetCode: string;
  operationType: InvestmentOperationType | '';
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

interface PositionPreviewRow {
  assetCode: string;
  assetName: string;
  currentQuantity: number;
  draftChange: number;
  projectedQuantity: number;
  currentAveragePrice: number;
  projectedAveragePrice: number;
}

const INITIAL_ROWS = 10;
const INSERT_COLUMN_COUNT = 6;

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
      <a routerLink="/investments/positions" routerLinkActive="active">{{ nav.positions }}</a>
      <a routerLink="/investments/assets" routerLinkActive="active">{{ nav.assets }}</a>
      <a routerLink="/investments/insert" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }">
        {{ nav.insert }}
      </a>
      <a routerLink="/investments/operations" routerLinkActive="active">{{ nav.operations }}</a>
      <a routerLink="/investments/portfolios" routerLinkActive="active">{{ nav.portfolios }}</a>
    </nav>

    <section class="panel">
      <div class="insert-table-shell">
        <div class="table-wrap insert-table-wrap">
          <table class="insert-table">
            <colgroup>
              <col class="insert-col-date" />
              <col class="insert-col-description" />
              <col class="insert-col-type" />
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
                      (blur)="finishDateEdit(row)"
                      (paste)="handlePaste($event, rowIndex, 0)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td>
                    <input
                      class="grid-input"
                      data-grid-cell
                      [placeholder]="messages.placeholders.asset"
                      [(ngModel)]="row.assetCode"
                      name="asset-{{ row.id }}"
                      (blur)="normalizeAsset(row)"
                      (paste)="handlePaste($event, rowIndex, 1)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td>
                    <select
                      class="grid-input compact-grid-input"
                      data-grid-cell
                      [(ngModel)]="row.operationType"
                      name="type-{{ row.id }}"
                      (paste)="handlePaste($event, rowIndex, 2)"
                      (keydown)="moveCell($event)"
                    >
                      <option value=""></option>
                      <option value="BUY">{{ messages.types.buy }}</option>
                      <option value="SELL">{{ messages.types.sell }}</option>
                      <option value="BONIFICATION">{{ messages.types.bonification }}</option>
                    </select>
                  </td>
                  <td>
                    <input
                      class="grid-input compact-grid-input"
                      data-grid-cell
                      [placeholder]="messages.placeholders.quantity"
                      inputmode="numeric"
                      [(ngModel)]="row.quantity"
                      name="quantity-{{ row.id }}"
                      (blur)="normalizeQuantity(row)"
                      (paste)="handlePaste($event, rowIndex, 3)"
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
                      (blur)="finishMoneyEdit(row, 'unitPrice')"
                      (paste)="handlePaste($event, rowIndex, 4)"
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
                      (blur)="finishMoneyEdit(row, 'totalFeeAmount')"
                      (paste)="handlePaste($event, rowIndex, 5)"
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
                    <td colspan="7">{{ rowValidation(row).errors.join(' · ') }}</td>
                  </tr>
                }
                @if (rowGroupingWarning(row); as warning) {
                  <tr class="draft-warning-row">
                    <td colspan="7">{{ warning }}</td>
                  </tr>
                }
              }
            </tbody>
          </table>
        </div>

        @if (positionPreviewRows().length > 0) {
          <div class="insert-preview-block">
            <div class="panel-header insert-preview-header">
              <h2>{{ messages.preview.title }}</h2>
            </div>
            <div class="table-wrap insert-preview-wrap">
              <table class="insert-preview-table">
                <thead>
                  <tr>
                    <th>{{ messages.preview.asset }}</th>
                    <th>{{ messages.preview.currentQuantity }}</th>
                    <th>{{ messages.preview.draftChange }}</th>
                    <th>{{ messages.preview.projectedQuantity }}</th>
                    <th>{{ messages.preview.currentAveragePrice }}</th>
                    <th>{{ messages.preview.projectedAveragePrice }}</th>
                  </tr>
                </thead>
                <tbody>
                  @for (row of positionPreviewRows(); track row.assetCode) {
                    <tr>
                      <td>{{ assetLabel(row.assetCode, row.assetName) }}</td>
                      <td class="amount-cell">{{ row.currentQuantity }}</td>
                      <td class="amount-cell">{{ row.draftChange }}</td>
                      <td class="amount-cell"><strong>{{ row.projectedQuantity }}</strong></td>
                      <td class="amount-cell">{{ money(row.currentAveragePrice) }}</td>
                      <td class="amount-cell"><strong>{{ money(row.projectedAveragePrice) }}</strong></td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          </div>
        }
      </div>
    </section>
  `,
})
export class InvestmentInsertComponent implements OnInit, AfterViewInit {
  private readonly moneyVisibility = inject(MoneyVisibilityService);
  readonly nav = uiMessages.investments.nav;
  readonly messages = uiMessages.investments.insert;
  readonly rows = signal<DraftOperationRow[]>([]);
  readonly saving = signal(false);
  readonly assets = signal<InvestmentAsset[]>([]);
  readonly positions = signal<InvestmentPosition[]>([]);
  readonly editingDateRowId = signal<number | null>(null);

  private nextId = 1;

  constructor(
    private readonly investmentsService: InvestmentsService,
    private readonly toast: ToastService,
    private readonly elementRef: ElementRef<HTMLElement>,
  ) {}

  ngOnInit(): void {
    this.resetRows();
    forkJoin({
      assets: this.investmentsService.listAssets(),
      positions: this.investmentsService.listPositions(),
    }).subscribe({
      next: ({ assets, positions }) => {
        this.assets.set(assets);
        this.positions.set(positions);
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
    row.date = formatDraftDateInput(value);
    this.rows.update((rows) => [...rows]);
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
    row.date = normalizeDraftDate(row.date);
    this.editingDateRowId.set(null);
    this.rows.update((rows) => [...rows]);
  }

  dateInputValue(row: DraftOperationRow): string {
    return row.date;
  }

  normalizeDate(row: DraftOperationRow): void {
    row.date = normalizeDraftDate(row.date);
    this.rows.update((rows) => [...rows]);
  }

  normalizeAsset(row: DraftOperationRow): void {
    row.assetCode = row.assetCode.trim().toUpperCase();
    this.rows.update((rows) => [...rows]);
  }

  normalizeQuantity(row: DraftOperationRow): void {
    row.quantity = row.quantity.replace(/[^\d]/g, '');
    this.rows.update((rows) => [...rows]);
  }

  onMoneyInput(row: DraftOperationRow, field: 'unitPrice' | 'totalFeeAmount', value: string): void {
    const manualFlag = this.moneyManualFlag(field);
    if (!value.trim()) {
      row[field] = '';
      row[manualFlag] = false;
      this.rows.update((rows) => [...rows]);
      return;
    }

    if (row[manualFlag]) {
      row[field] = value;
      if (!value.includes(',')) {
        row[manualFlag] = false;
      }
      this.rows.update((rows) => [...rows]);
      return;
    }

    row[field] = formatAmountDigitsAsCents(value);
    this.rows.update((rows) => [...rows]);
  }

  finishMoneyEdit(row: DraftOperationRow, field: 'unitPrice' | 'totalFeeAmount'): void {
    row[field] = normalizeDraftAmount(row[field]);
    const manualFlag = this.moneyManualFlag(field);
    row[manualFlag] = row[field].includes(',');
    this.rows.update((rows) => [...rows]);
  }

  normalizeMoney(row: DraftOperationRow, field: 'unitPrice' | 'totalFeeAmount'): void {
    if (!row[field].trim()) {
      row[field] = '';
      row[this.moneyManualFlag(field)] = false;
      this.rows.update((rows) => [...rows]);
      return;
    }
    row[field] = normalizeDraftAmount(row[field]);
    row[this.moneyManualFlag(field)] = row[field].includes(',');
    this.rows.update((rows) => [...rows]);
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
      .map((line) => line.split('\t'));
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

    return { valid: errors.length === 0, errors };
  }

  rowGroupingWarning(row: DraftOperationRow): string {
    if (this.isEmpty(row)) {
      return '';
    }

    const dateKey = normalizedDateKey(row.date);
    if (!dateKey) {
      return '';
    }

    const sameDateRows = this.filledRows().filter((candidate) => normalizedDateKey(candidate.date) === dateKey);
    const distinctFeeValues = Array.from(
      new Set(sameDateRows.map((candidate) => decimalToCents(candidate.totalFeeAmount))),
    );
    if (distinctFeeValues.length <= 1) {
      return '';
    }

    return 'Taxas diferentes nesta data serão rateadas em grupos separados.';
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
    this.saving.set(true);
    this.investmentsService.createBulkOperations({
      operations: filled.map((row) => ({
        asset_code: row.assetCode,
        operation_type: row.operationType as InvestmentOperationType,
        date: dateInputToIso(brazilianDateToQuery(row.date)),
        quantity: Number(row.quantity),
        unit_price: decimalToCents(row.unitPrice),
        total_fee_amount: decimalToCents(row.totalFeeAmount),
        notes: '',
      })),
    }).subscribe({
      next: () => {
        this.toast.success('Operações salvas.');
        this.resetRows();
        forkJoin({
          assets: this.investmentsService.listAssets(),
          positions: this.investmentsService.listPositions(),
        }).subscribe({
          next: ({ assets, positions }) => {
            this.assets.set(assets);
            this.positions.set(positions);
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

  positionPreviewRows(): PositionPreviewRow[] {
    const validRows = this.filledRows()
      .filter((row) => this.rowValidation(row).valid)
      .map((row, index) => ({
        row,
        index,
        asset: this.assetByCode(row.assetCode),
        dateKey: normalizedDateKey(row.date)!,
        quantity: Number(row.quantity),
        unitPrice: decimalToCents(row.unitPrice),
        totalFeeAmount: decimalToCents(row.totalFeeAmount),
      }));

    if (validRows.length === 0) {
      return [];
    }

    const allocatedFees = allocatePreviewFees(validRows);
    const currentByAsset = new Map<string, { name: string; quantity: number; costBasis: number }>();
    for (const position of this.positions()) {
      currentByAsset.set(position.asset_code, {
        name: position.asset_name,
        quantity: position.current_quantity,
        costBasis: position.total_cost_basis,
      });
    }

    validRows.sort((left, right) => {
      const leftDate = brazilianDateToQuery(left.row.date);
      const rightDate = brazilianDateToQuery(right.row.date);
      if (leftDate === rightDate) {
        return left.index - right.index;
      }
      return leftDate.localeCompare(rightDate);
    });

    const projectedByAsset = new Map<string, PositionPreviewRow>();
    for (const item of validRows) {
      const assetCode = item.row.assetCode;
      const current = projectedByAsset.get(assetCode) ?? (() => {
        const existing = currentByAsset.get(assetCode);
        return {
          assetCode,
          assetName: item.asset?.name ?? assetCode,
          currentQuantity: existing?.quantity ?? 0,
          draftChange: 0,
          projectedQuantity: existing?.quantity ?? 0,
          currentAveragePrice: existing?.quantity ? divideRounded(existing.costBasis, existing.quantity) : 0,
          projectedAveragePrice: existing?.quantity ? divideRounded(existing.costBasis, existing.quantity) : 0,
        } satisfies PositionPreviewRow;
      })();

      let projectedCostBasis = current.projectedQuantity * current.projectedAveragePrice;
      const grossAmount = item.quantity * item.unitPrice;
      const allocatedFee = allocatedFees.get(item.row.id) ?? 0;
      if (item.row.operationType === 'BUY' || item.row.operationType === 'BONIFICATION') {
        current.projectedQuantity += item.quantity;
        projectedCostBasis += grossAmount + allocatedFee;
      } else if (item.row.operationType === 'SELL') {
        if (current.projectedQuantity > 0) {
          const sellCostBasis = divideRounded(projectedCostBasis*item.quantity, current.projectedQuantity);
          current.projectedQuantity -= item.quantity;
          projectedCostBasis -= sellCostBasis;
        } else {
          current.projectedQuantity -= item.quantity;
          projectedCostBasis = 0;
        }
      }
      current.draftChange = current.projectedQuantity - current.currentQuantity;
      current.projectedAveragePrice = current.projectedQuantity > 0 ? divideRounded(projectedCostBasis, current.projectedQuantity) : 0;
      projectedByAsset.set(assetCode, current);
    }

    return Array.from(projectedByAsset.values()).sort((left, right) => left.assetCode.localeCompare(right.assetCode));
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrency(value);
  }

  assetLabel(code: string, name: string): string {
    return investmentAssetLabel(code, name);
  }

  private filledRows(): DraftOperationRow[] {
    return this.rows().filter((row) => !this.isEmpty(row));
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

  private createEmptyRow(id = this.nextId++): DraftOperationRow {
    return {
      id,
      date: '',
      assetCode: '',
      operationType: '',
      quantity: '',
      unitPrice: '',
      unitPriceManualDecimal: false,
      totalFeeAmount: '0,00',
      totalFeeAmountManualDecimal: false,
    };
  }

  private isEmpty(row: DraftOperationRow): boolean {
    return !row.date && !row.assetCode && !row.operationType && !row.quantity && !row.unitPrice;
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
        row.quantity = value;
        break;
      case 4:
        row.unitPrice = value;
        row.unitPriceManualDecimal = value.includes(',');
        break;
      case 5:
        row.totalFeeAmount = value;
        row.totalFeeAmountManualDecimal = value.includes(',');
        break;
    }
  }

  private moneyManualFlag(field: 'unitPrice' | 'totalFeeAmount'): 'unitPriceManualDecimal' | 'totalFeeAmountManualDecimal' {
    return field === 'unitPrice' ? 'unitPriceManualDecimal' : 'totalFeeAmountManualDecimal';
  }

  private assetByCode(code: string): InvestmentAsset | undefined {
    const normalized = code.trim().toUpperCase();
    return this.assets().find((asset) => asset.code === normalized);
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
  if (!value.trim()) {
    return '';
  }
  const cents = decimalToCents(value);
  return centsToDecimal(cents).replace('.', ',');
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

function allocatePreviewFees(
  rows: Array<{
    row: DraftOperationRow;
    dateKey: string;
    quantity: number;
    unitPrice: number;
    totalFeeAmount: number;
  }>,
): Map<number, number> {
  const result = new Map<number, number>();
  const grouped = new Map<string, typeof rows>();
  for (const row of rows) {
    const groupKey = `${row.dateKey}:${row.totalFeeAmount}`;
    const current = grouped.get(groupKey) ?? [];
    current.push(row);
    grouped.set(groupKey, current);
  }
  for (const groupRows of grouped.values()) {
    const dayFee = groupRows[0]?.totalFeeAmount ?? 0;
    const totalGross = groupRows.reduce((sum, row) => sum + row.quantity * row.unitPrice, 0);
    if (dayFee <= 0 || totalGross <= 0) {
      groupRows.forEach((row) => result.set(row.row.id, 0));
      continue;
    }
    let remaining = dayFee;
    groupRows.forEach((row, index) => {
      if (index === groupRows.length - 1) {
        result.set(row.row.id, remaining);
        return;
      }
      const allocated = Math.min(remaining, divideRounded(dayFee * row.quantity * row.unitPrice, totalGross));
      result.set(row.row.id, allocated);
      remaining -= allocated;
    });
  }
  return result;
}

function divideRounded(numerator: number, denominator: number): number {
  if (!denominator) {
    return 0;
  }
  return Math.round(numerator / denominator);
}
