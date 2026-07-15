import { AfterViewInit, Component, ElementRef, OnInit, computed, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { ReferenceDataService } from '../../data/reference-data.service';
import { TransactionsService } from '../../data/transactions.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { uiMessages, insertTransactionsSuccessMessage } from '../../shared/messages';
import { brazilianDateToQuery, dateInputToIso, decimalToCents } from '../../shared/money';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { Category, Suggestion, SuggestionEntryType, TransactionPayload } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

type EntryType = 'REVENUE' | 'EXPENSE' | 'TRANSFER' | '';

interface DraftTransactionRow {
  id: number;
  description: string;
  notes: string;
  amount: string;
  amountManualDecimal: boolean;
  type: EntryType;
  typeLabel: string;
  date: string;
  dateAutoFilled: boolean;
  dateManuallyEdited: boolean;
  category: string;
  accountCode: string;
  accountLabel: string;
  transferAccountCode: string;
  transferAccountLabel: string;
  excludeFromDashboard: boolean;
}

interface RowValidation {
  valid: boolean;
  errors: string[];
}

interface AccountPreviewRow {
  code: string;
  name: string;
  currentBalance: number;
  draftImpact: number;
  projectedBalance: number;
}

interface PickerOption {
  value: string;
  label: string;
  menuLabel?: string;
}

type MenuKind = 'type' | 'category' | 'account' | 'transfer' | null;

const INITIAL_ROWS = 10;
const INSERT_COLUMN_COUNT = 7;

@Component({
  selector: 'app-insert-transactions',
  imports: [FormsModule],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.page.eyebrow }}</p>
        <h1>{{ messages.page.title }}</h1>
      </div>
      <div class="insert-actions">
        <button class="ghost-button" type="button" (click)="addRows(10)">
          {{ messages.page.addRows }}
        </button>
        <button class="ghost-button" type="button" (click)="clearEmptyRows()">
          {{ messages.page.clearEmpty }}
        </button>
        <button
          class="primary-button"
          type="button"
          [disabled]="!canSubmit() || saving()"
          (click)="submit()"
        >
          {{ saving() ? messages.page.submitting : messages.page.submit }}
        </button>
      </div>
    </section>

    <section class="panel">
      <div class="insert-table-shell">
        <div class="table-wrap insert-table-wrap">
          <table class="insert-table">
            <colgroup>
              <col class="insert-col-description" />
              <col class="insert-col-amount" />
              <col class="insert-col-date" />
              <col class="insert-col-type" />
              <col class="insert-col-category" />
              <col class="insert-col-account" />
              <col class="insert-col-transfer" />
              <col class="insert-col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>{{ messages.columns.description }}</th>
                <th>{{ messages.columns.amount }}</th>
                <th>{{ messages.columns.date }}</th>
                <th>{{ messages.columns.type }}</th>
                <th>{{ messages.columns.category }}</th>
                <th>{{ messages.columns.account }}</th>
                <th>{{ messages.columns.transferAccount }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              @for (row of rows(); track row.id; let rowIndex = $index) {
                <tr
                  [class.invalid-draft-row]="rowValidation(row).errors.length > 0"
                  [class.category-menu-row]="isAnyMenuOpenForRow(row.id)"
                >
                  <td>
                    <input
                      class="grid-input"
                      data-grid-cell
                      name="description-{{ row.id }}"
                      [(ngModel)]="row.description"
                      (ngModelChange)="onDescriptionChange(row)"
                      (blur)="finishDescriptionEdit(row)"
                      (paste)="handlePaste($event, rowIndex, 0)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td>
                    <input
                      class="grid-input amount-draft-input"
                      data-grid-cell
                      name="amount-{{ row.id }}"
                      inputmode="decimal"
                      [placeholder]="messages.placeholders.amount"
                      [(ngModel)]="row.amount"
                      (ngModelChange)="onAmountInput(row, $event)"
                      (blur)="finishAmountEdit(row)"
                      (paste)="handlePaste($event, rowIndex, 1)"
                      (keydown)="handleAmountKeydown($event, row); moveCell($event)"
                    />
                  </td>
                  <td>
                    <input
                      class="grid-input compact-grid-input date-draft-input"
                      data-grid-cell
                      name="date-{{ row.id }}"
                      inputmode="numeric"
                      [placeholder]="messages.placeholders.date"
                      [ngModel]="dateInputValue(row)"
                      (focus)="startDateEdit(row, $any($event.target))"
                      (ngModelChange)="onDateInput(row, $event)"
                      (blur)="finishDateEdit(row)"
                      (paste)="handlePaste($event, rowIndex, 2)"
                      (keydown)="moveCell($event)"
                    />
                  </td>
                  <td>
                    <div class="category-input-wrap" (mouseenter)="cancelTypeMenuClose()">
                      <input
                        class="grid-input compact-grid-input"
                        data-grid-cell
                        autocomplete="off"
                        [attr.data-menu-anchor]="'type-' + row.id"
                        name="type-{{ row.id }}"
                        [(ngModel)]="row.typeLabel"
                        (focus)="openTypeMenu(row.id)"
                        (click)="openTypeMenu(row.id)"
                        (ngModelChange)="onTypeInputChange(row)"
                        (blur)="finishTypeEdit(row)"
                        (paste)="handlePaste($event, rowIndex, 3)"
                        (keydown)="handleTypeKeydown($event, row); moveCell($event)"
                      />
                    </div>
                  </td>
                  <td>
                    <div class="category-input-wrap" (mouseenter)="cancelCategoryMenuClose()">
                      <input
                        class="grid-input compact-grid-input"
                        data-grid-cell
                        autocomplete="off"
                        [attr.data-menu-anchor]="'category-' + row.id"
                        name="category-{{ row.id }}"
                        [(ngModel)]="row.category"
                        (focus)="openCategoryMenu(row.id)"
                        (click)="openCategoryMenu(row.id)"
                        (ngModelChange)="onCategoryInputChange(row)"
                        (blur)="finishCategoryEdit(row)"
                        (paste)="handlePaste($event, rowIndex, 4)"
                        (keydown)="handleCategoryKeydown($event, row); moveCell($event)"
                      />
                    </div>
                  </td>
                  <td>
                    <div class="category-input-wrap" (mouseenter)="cancelAccountMenuClose()">
                      <input
                        class="grid-input compact-grid-input"
                        data-grid-cell
                        autocomplete="off"
                        [attr.data-menu-anchor]="'account-' + row.id"
                        name="account-{{ row.id }}"
                        [(ngModel)]="row.accountLabel"
                        (focus)="openAccountMenu(row.id)"
                        (click)="openAccountMenu(row.id)"
                        (ngModelChange)="onAccountInputChange(row)"
                        (blur)="finishAccountEdit(row)"
                        (paste)="handlePaste($event, rowIndex, 5)"
                        (keydown)="handleAccountKeydown($event, row); moveCell($event)"
                      />
                    </div>
                  </td>
                  <td>
                    @if (row.type === 'TRANSFER') {
                      <div class="category-input-wrap" (mouseenter)="cancelTransferMenuClose()">
                        <input
                          class="grid-input compact-grid-input"
                          data-grid-cell
                          autocomplete="off"
                          [attr.data-menu-anchor]="'transfer-' + row.id"
                          name="transfer-{{ row.id }}"
                          [(ngModel)]="row.transferAccountLabel"
                          (focus)="openTransferMenu(row.id)"
                          (click)="openTransferMenu(row.id)"
                          (ngModelChange)="onTransferInputChange(row)"
                          (blur)="finishTransferEdit(row)"
                          (paste)="handlePaste($event, rowIndex, 6)"
                          (keydown)="handleTransferKeydown($event, row); moveCell($event)"
                        />
                      </div>
                    } @else {
                      <span class="draft-muted">-</span>
                    }
                  </td>
                  <td class="actions-cell transaction-insert-actions-cell">
                    <div class="transaction-insert-actions">
                      <button
                        class="icon-action"
                        type="button"
                        [title]="messages.actions.duplicateTitle"
                        [attr.aria-label]="messages.actions.duplicateAria"
                        (click)="duplicateRow(rowIndex)"
                      >
                        ⧉
                      </button>
                      <button
                        class="icon-action"
                        type="button"
                        [title]="messages.actions.addBelowTitle"
                        [attr.aria-label]="messages.actions.addBelowAria"
                        (click)="addRowAfter(rowIndex)"
                      >
                        +
                      </button>
                      <button
                        class="icon-action notes-action"
                        type="button"
                        [class.has-notes]="row.notes.trim().length > 0"
                        [title]="messages.actions.notesTitle"
                        [attr.aria-label]="messages.actions.notesAria"
                        (click)="$event.stopPropagation(); openNotesModal(row)"
                      >
                        📝
                      </button>
                      <button
                        class="icon-action"
                        type="button"
                        [title]="messages.actions.clearTitle"
                        [attr.aria-label]="messages.actions.clearAria"
                        (click)="clearRow(row)"
                      >
                        <svg aria-hidden="true" viewBox="0 0 24 24">
                          <path
                            d="M9 3h6l1 2h4v2H4V5h4l1-2Zm-1 6h2v10H8V9Zm6 0h2v10h-2V9Zm-9 0h14l-1 12H6L5 9Z"
                          />
                        </svg>
                      </button>
                    </div>
                  </td>
                </tr>
                @if (rowValidation(row).errors.length > 0) {
                  <tr class="draft-error-row">
                    <td colspan="8">{{ rowValidation(row).errors.join(' · ') }}</td>
                  </tr>
                }
              }
            </tbody>
          </table>
        </div>
        @if (accountPreviewRows().length > 0) {
          <div class="insert-preview-block">
            <div class="panel-header insert-preview-header">
              <h2>{{ messages.preview.title }}</h2>
            </div>
            <div class="table-wrap insert-preview-wrap">
              <table class="insert-preview-table">
                <thead>
                  <tr>
                    <th>{{ messages.preview.account }}</th>
                    <th>{{ messages.preview.currentBalance }}</th>
                    <th>{{ messages.preview.impact }}</th>
                    <th>{{ messages.preview.projectedBalance }}</th>
                  </tr>
                </thead>
                <tbody>
                  @for (account of accountPreviewRows(); track account.code) {
                    <tr>
                      <td>{{ account.name }}</td>
                      <td class="amount-cell">{{ money(account.currentBalance) }}</td>
                      <td class="amount-cell">{{ money(account.draftImpact) }}</td>
                      <td class="amount-cell">
                        <strong>{{ money(account.projectedBalance) }}</strong>
                      </td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          </div>
        }
        @if (activeMenuKind() && activeMenuOptions().length > 0) {
          <div
            class="category-menu shell-menu"
            [class.category-shell-menu]="activeMenuKind() === 'category'"
            [style.top.px]="menuPosition().top"
            [style.left.px]="menuPosition().left"
            [style.width.px]="menuPosition().width"
          >
            @for (option of activeMenuOptions(); track option.value; let optionIndex = $index) {
              <button
                class="category-option"
                [class.active]="activeMenuActiveIndex() === optionIndex"
                [attr.data-menu-option]="optionIndex"
                type="button"
                (mousedown)="selectActiveMenuOption(option, $event)"
              >
                {{ option.menuLabel ?? option.label }}
              </button>
            }
          </div>
        }
      </div>
    </section>

    @if (notesModalOpen()) {
      <div class="modal-backdrop" (click)="closeNotesModal()">
        <section class="panel notes-modal" (click)="$event.stopPropagation()">
          <div class="panel-header">
            <div>
              <h2>{{ messages.notesModal.title }}</h2>
              <p>{{ messages.notesModal.subtitle }}</p>
            </div>
            <button class="ghost-button" type="button" (click)="closeNotesModal()">{{ messages.notesModal.close }}</button>
          </div>
          <label class="notes-field">
            {{ messages.notesModal.label }}
            <textarea
              rows="8"
              [ngModel]="notesDraft()"
              (ngModelChange)="notesDraft.set($event)"
              [placeholder]="messages.notesModal.placeholder"
            ></textarea>
          </label>
          <div class="notes-modal-actions">
            <button class="ghost-button" type="button" (click)="closeNotesModal()">{{ messages.notesModal.cancel }}</button>
            <button class="primary-button" type="button" (click)="saveNotes()">{{ messages.notesModal.save }}</button>
          </div>
        </section>
      </div>
    }
  `,
})
export class InsertTransactionsComponent implements OnInit, AfterViewInit {
  readonly messages = uiMessages.insert;
  readonly rows = signal<DraftTransactionRow[]>([]);
  readonly saving = signal(false);
  readonly error = signal('');
  readonly success = signal('');
  readonly editingDateRowId = signal<number | null>(null);
  readonly categoryMenuRowId = signal<number | null>(null);
  readonly categoryMenuActiveIndex = signal(0);
  readonly typeMenuRowId = signal<number | null>(null);
  readonly typeMenuActiveIndex = signal(0);
  readonly accountMenuRowId = signal<number | null>(null);
  readonly accountMenuActiveIndex = signal(0);
  readonly transferMenuRowId = signal<number | null>(null);
  readonly transferMenuActiveIndex = signal(0);
  readonly menuOpenUpward = signal(false);
  readonly menuPosition = signal({ top: 0, left: 0, width: 160 });
  readonly activeMenuKind = signal<MenuKind>(null);
  readonly notesModalRowId = signal<number | null>(null);
  readonly notesDraft = signal('');
  readonly notesModalOpen = computed(() => this.notesModalRowId() !== null);

  private nextId = 1;
  private closeCategoryMenuTimer: ReturnType<typeof setTimeout> | null = null;
  private closeTypeMenuTimer: ReturnType<typeof setTimeout> | null = null;
  private closeAccountMenuTimer: ReturnType<typeof setTimeout> | null = null;
  private closeTransferMenuTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(
    readonly referenceData: ReferenceDataService,
    private readonly transactionsService: TransactionsService,
    private readonly elementRef: ElementRef<HTMLElement>,
    private readonly moneyVisibility: MoneyVisibilityService,
    private readonly toast: ToastService,
  ) {}

  ngOnInit(): void {
    this.resetRows();
    this.referenceData.load().subscribe({
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  ngAfterViewInit(): void {
    this.focusFirstDescriptionInput();
  }

  addRows(count: number): void {
    this.rows.update((rows) => [
      ...rows,
      ...Array.from({ length: count }, () => this.createEmptyRow()),
    ]);
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

  clearRow(row: DraftTransactionRow): void {
    Object.assign(row, this.createEmptyRow(row.id));
    this.rows.update((rows) => [...rows]);
  }

  clearEmptyRows(): void {
    const nonEmptyRows = this.rows().filter((row) => !this.isEmpty(row));
    this.rows.set(nonEmptyRows.length > 0 ? nonEmptyRows : [this.createEmptyRow()]);
  }

  onDescriptionChange(row: DraftTransactionRow): void {
    if (row.description.trim() && !row.date) {
      row.date = todayBrazilianDate();
      row.dateAutoFilled = true;
      row.dateManuallyEdited = false;
    }
  }

  finishDescriptionEdit(row: DraftTransactionRow): void {
    this.applySuggestionToRow(row);
    this.rows.update((rows) => [...rows]);
  }

  onDateInput(row: DraftTransactionRow, value: string): void {
    row.date = formatDraftDateInput(value);
    row.dateAutoFilled = false;
    row.dateManuallyEdited = true;
  }

  finishAmountEdit(row: DraftTransactionRow): void {
    row.amount = normalizeDraftAmount(row.amount);
    row.amountManualDecimal = row.amount.includes(',');
    this.rows.update((rows) => [...rows]);
  }

  onAmountInput(row: DraftTransactionRow, value: string): void {
    if (!value.trim()) {
      row.amount = '';
      row.amountManualDecimal = false;
      this.rows.update((rows) => [...rows]);
      return;
    }

    if (row.amountManualDecimal) {
      row.amount = value;
      if (!value.includes(',')) {
        row.amountManualDecimal = false;
      }
      this.rows.update((rows) => [...rows]);
      return;
    }

    row.amount = formatAmountDigitsAsCents(value);
    this.rows.update((rows) => [...rows]);
  }

  onTypeChange(row: DraftTransactionRow): void {
    row.category = '';
    if (row.type !== 'TRANSFER') {
      row.transferAccountCode = '';
      row.transferAccountLabel = '';
    }
  }

  onTypeInputChange(row: DraftTransactionRow): void {
    this.cancelTypeMenuClose();
    this.typeMenuRowId.set(row.id);
    this.typeMenuActiveIndex.set(0);
    row.type = (this.matchedTypeOption(row)?.value as EntryType) ?? '';
  }

  onAccountInputChange(row: DraftTransactionRow): void {
    this.cancelAccountMenuClose();
    this.accountMenuRowId.set(row.id);
    this.accountMenuActiveIndex.set(0);
    row.accountCode = this.matchedAccountOption(row)?.value ?? '';
  }

  onTransferInputChange(row: DraftTransactionRow): void {
    this.cancelTransferMenuClose();
    this.transferMenuRowId.set(row.id);
    this.transferMenuActiveIndex.set(0);
    row.transferAccountCode = this.matchedTransferOption(row)?.value ?? '';
  }

  onCategoryInputChange(row: DraftTransactionRow): void {
    this.cancelCategoryMenuClose();
    this.categoryMenuRowId.set(row.id);
    this.categoryMenuActiveIndex.set(0);
  }

  startDateEdit(row: DraftTransactionRow, input?: HTMLInputElement): void {
    this.editingDateRowId.set(row.id);
    if (!row.dateAutoFilled || row.dateManuallyEdited || !input) {
      return;
    }
    setTimeout(() => {
      if (this.editingDateRowId() === row.id) {
        input.select();
        input.setSelectionRange(0, input.value.length);
      }
    }, 0);
  }

  finishDateEdit(row: DraftTransactionRow): void {
    row.date = normalizeDraftDate(row.date);
    this.editingDateRowId.set(null);
    this.rows.update((rows) => [...rows]);
  }

  dateInputValue(row: DraftTransactionRow): string {
    return this.editingDateRowId() === row.id ? row.date : this.compactDateLabel(row);
  }

  compactDateLabel(row: DraftTransactionRow): string {
    if (!row.date.trim()) {
      return '--';
    }

    const normalized = normalizeDraftDate(row.date);
    if (!isBrazilianDate(normalized)) {
      return row.date;
    }

    return compactBrazilianDate(normalized);
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
      this.elementRef.nativeElement.querySelectorAll<HTMLElement>(
        '[data-grid-cell]:not([disabled])',
      ),
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

  handlePaste(event: ClipboardEvent, startRowIndex: number, startColumnIndex: number): void {
    const text = event.clipboardData?.getData('text/plain') ?? '';
    if (!text) {
      return;
    }

    event.preventDefault();
    this.ensureRowCount(startRowIndex + this.pasteRows(text).length);
    this.closeAllMenus();

    const rows = this.rows();
    this.pasteRows(text).forEach((values, rowOffset) => {
      const row = rows[startRowIndex + rowOffset];
      if (!row) {
        return;
      }

      values.slice(0, INSERT_COLUMN_COUNT - startColumnIndex).forEach((value, columnOffset) => {
        if (!value.trim()) {
          return;
        }
        this.applyPastedValue(row, startColumnIndex + columnOffset, value);
      });

      this.normalizeRowAfterPaste(row);
    });

    this.rows.update((currentRows) => [...currentRows]);
  }

  categoryOptions(row: DraftTransactionRow): Category[] {
    const leafCategories = this.leafCategories();
    if (row.type === 'TRANSFER') {
      return leafCategories.filter((category) => category.Type === 'MOVEMENT');
    }
    return leafCategories.filter((category) => category.Type !== 'MOVEMENT');
  }

  suggestedCategoryOptions(row: DraftTransactionRow): Category[] {
    const options = this.categoryOptions(row);
    const normalized = normalize(row.category);
    if (!normalized) {
      return options;
    }

    const startsWith = options.filter((category) =>
      normalize(category.Name).startsWith(normalized),
    );
    if (startsWith.length > 0) {
      return startsWith;
    }

    const includes = options.filter((category) => normalize(category.Name).includes(normalized));
    return includes.length > 0 ? includes : options;
  }

  typeOptions(): PickerOption[] {
    return [
      { value: 'REVENUE', label: this.messages.types.revenue },
      { value: 'EXPENSE', label: this.messages.types.expense },
      { value: 'TRANSFER', label: this.messages.types.transfer },
    ];
  }

  accountOptions(row: DraftTransactionRow): PickerOption[] {
    return this.referenceData
      .accounts()
      .filter((account) => account.Code !== row.transferAccountCode)
      .map((account) => ({ value: account.Code, label: account.Name }));
  }

  transferOptions(row: DraftTransactionRow): PickerOption[] {
    return this.referenceData
      .accounts()
      .filter((account) => account.Code !== row.accountCode)
      .map((account) => ({ value: account.Code, label: account.Name }));
  }

  suggestedTypeOptions(row: DraftTransactionRow): PickerOption[] {
    return this.filterPickerOptions(this.typeOptions(), row.typeLabel);
  }

  suggestedAccountOptions(row: DraftTransactionRow): PickerOption[] {
    return this.filterPickerOptions(this.accountOptions(row), row.accountLabel);
  }

  suggestedTransferOptions(row: DraftTransactionRow): PickerOption[] {
    return this.filterPickerOptions(this.transferOptions(row), row.transferAccountLabel);
  }

  activeMenuOptions(): PickerOption[] {
    const row = this.activeMenuRow();
    if (!row) {
      return [];
    }
    switch (this.activeMenuKind()) {
      case 'type':
        return this.suggestedTypeOptions(row);
      case 'category':
        return this.suggestedCategoryOptions(row).map((category) => ({
          value: category.Code,
          label: category.Name,
          menuLabel: this.categoryPickerMenuLabel(category),
        }));
      case 'account':
        return this.suggestedAccountOptions(row);
      case 'transfer':
        return this.suggestedTransferOptions(row);
      default:
        return [];
    }
  }

  activeMenuActiveIndex(): number {
    switch (this.activeMenuKind()) {
      case 'type':
        return this.typeMenuActiveIndex();
      case 'category':
        return this.categoryMenuActiveIndex();
      case 'account':
        return this.accountMenuActiveIndex();
      case 'transfer':
        return this.transferMenuActiveIndex();
      default:
        return 0;
    }
  }

  openCategoryMenu(rowId: number): void {
    this.cancelCategoryMenuClose();
    this.closeOtherMenus('category');
    this.activeMenuKind.set('category');
    if (this.categoryMenuRowId() !== rowId) {
      this.categoryMenuActiveIndex.set(0);
    }
    this.categoryMenuRowId.set(rowId);
    queueMicrotask(() => {
      this.updateMenuDirection(`category-${rowId}`);
      this.scrollActiveCategoryOptionIntoView(rowId);
    });
  }

  openTypeMenu(rowId: number): void {
    this.cancelTypeMenuClose();
    this.closeOtherMenus('type');
    this.activeMenuKind.set('type');
    if (this.typeMenuRowId() !== rowId) {
      this.typeMenuActiveIndex.set(0);
    }
    this.typeMenuRowId.set(rowId);
    queueMicrotask(() => {
      this.updateMenuDirection(`type-${rowId}`);
      this.scrollPickerOptionIntoView('type', rowId, this.typeMenuActiveIndex());
    });
  }

  openAccountMenu(rowId: number): void {
    this.cancelAccountMenuClose();
    this.closeOtherMenus('account');
    this.activeMenuKind.set('account');
    if (this.accountMenuRowId() !== rowId) {
      this.accountMenuActiveIndex.set(0);
    }
    this.accountMenuRowId.set(rowId);
    queueMicrotask(() => {
      this.updateMenuDirection(`account-${rowId}`);
      this.scrollPickerOptionIntoView('account', rowId, this.accountMenuActiveIndex());
    });
  }

  openTransferMenu(rowId: number): void {
    this.cancelTransferMenuClose();
    this.closeOtherMenus('transfer');
    this.activeMenuKind.set('transfer');
    if (this.transferMenuRowId() !== rowId) {
      this.transferMenuActiveIndex.set(0);
    }
    this.transferMenuRowId.set(rowId);
    queueMicrotask(() => {
      this.updateMenuDirection(`transfer-${rowId}`);
      this.scrollPickerOptionIntoView('transfer', rowId, this.transferMenuActiveIndex());
    });
  }

  selectCategoryOption(row: DraftTransactionRow, category: Category, event: MouseEvent): void {
    event.preventDefault();
    row.category = category.Name;
    this.categoryMenuRowId.set(null);
    this.rows.update((rows) => [...rows]);
  }

  selectTypeOption(row: DraftTransactionRow, option: PickerOption, event: MouseEvent): void {
    event.preventDefault();
    this.setRowType(row, option.value as EntryType, option.label);
    this.typeMenuRowId.set(null);
    this.rows.update((rows) => [...rows]);
  }

  selectAccountOption(row: DraftTransactionRow, option: PickerOption, event: MouseEvent): void {
    event.preventDefault();
    row.accountCode = option.value;
    row.accountLabel = option.label;
    this.accountMenuRowId.set(null);
    this.rows.update((rows) => [...rows]);
  }

  selectTransferOption(row: DraftTransactionRow, option: PickerOption, event: MouseEvent): void {
    event.preventDefault();
    row.transferAccountCode = option.value;
    row.transferAccountLabel = option.label;
    this.transferMenuRowId.set(null);
    this.rows.update((rows) => [...rows]);
  }

  cancelCategoryMenuClose(): void {
    if (this.closeCategoryMenuTimer !== null) {
      clearTimeout(this.closeCategoryMenuTimer);
      this.closeCategoryMenuTimer = null;
    }
  }

  cancelTypeMenuClose(): void {
    if (this.closeTypeMenuTimer !== null) {
      clearTimeout(this.closeTypeMenuTimer);
      this.closeTypeMenuTimer = null;
    }
  }

  cancelAccountMenuClose(): void {
    if (this.closeAccountMenuTimer !== null) {
      clearTimeout(this.closeAccountMenuTimer);
      this.closeAccountMenuTimer = null;
    }
  }

  cancelTransferMenuClose(): void {
    if (this.closeTransferMenuTimer !== null) {
      clearTimeout(this.closeTransferMenuTimer);
      this.closeTransferMenuTimer = null;
    }
  }

  private closeOtherMenus(kind: 'category' | 'type' | 'account' | 'transfer'): void {
    if (kind !== 'category') this.categoryMenuRowId.set(null);
    if (kind !== 'type') this.typeMenuRowId.set(null);
    if (kind !== 'account') this.accountMenuRowId.set(null);
    if (kind !== 'transfer') this.transferMenuRowId.set(null);
  }

  selectActiveMenuOption(option: PickerOption, event: MouseEvent): void {
    event.preventDefault();
    const row = this.activeMenuRow();
    if (!row) {
      return;
    }
    switch (this.activeMenuKind()) {
      case 'type':
        this.selectTypeOption(row, option, event);
        break;
      case 'category':
        row.category = option.label;
        this.categoryMenuRowId.set(null);
        this.rows.update((rows) => [...rows]);
        break;
      case 'account':
        this.selectAccountOption(row, option, event);
        break;
      case 'transfer':
        this.selectTransferOption(row, option, event);
        break;
    }
    this.activeMenuKind.set(null);
  }

  handleCategoryKeydown(event: KeyboardEvent, row: DraftTransactionRow): void {
    const options = this.suggestedCategoryOptions(row);
    if (options.length === 0) {
      return;
    }

    if (event.key === 'ArrowDown') {
      event.preventDefault();
      this.openCategoryMenu(row.id);
      this.categoryMenuActiveIndex.update((index) => (index + 1) % options.length);
      queueMicrotask(() => this.scrollActiveCategoryOptionIntoView(row.id));
      return;
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault();
      this.openCategoryMenu(row.id);
      this.categoryMenuActiveIndex.update((index) => (index - 1 + options.length) % options.length);
      queueMicrotask(() => this.scrollActiveCategoryOptionIntoView(row.id));
      return;
    }

    if (event.key === 'Enter' && this.categoryMenuRowId() === row.id) {
      event.preventDefault();
      const option = options[this.categoryMenuActiveIndex()] ?? options[0];
      if (option) {
        row.category = option.Name;
        this.categoryMenuRowId.set(null);
        this.rows.update((rows) => [...rows]);
      }
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      this.categoryMenuRowId.set(null);
    }
  }

  handleTypeKeydown(event: KeyboardEvent, row: DraftTransactionRow): void {
    this.handlePickerKeydown(
      event,
      row.id,
      this.suggestedTypeOptions(row),
      this.typeMenuRowId,
      this.typeMenuActiveIndex,
      (option) => {
        this.setRowType(row, option.value as EntryType, option.label);
        this.rows.update((rows) => [...rows]);
      },
      this.openTypeMenu.bind(this),
      'type',
    );
  }

  handleAccountKeydown(event: KeyboardEvent, row: DraftTransactionRow): void {
    this.handlePickerKeydown(
      event,
      row.id,
      this.suggestedAccountOptions(row),
      this.accountMenuRowId,
      this.accountMenuActiveIndex,
      (option) => {
        row.accountCode = option.value;
        row.accountLabel = option.label;
        this.rows.update((rows) => [...rows]);
      },
      this.openAccountMenu.bind(this),
      'account',
    );
  }

  handleTransferKeydown(event: KeyboardEvent, row: DraftTransactionRow): void {
    this.handlePickerKeydown(
      event,
      row.id,
      this.suggestedTransferOptions(row),
      this.transferMenuRowId,
      this.transferMenuActiveIndex,
      (option) => {
        row.transferAccountCode = option.value;
        row.transferAccountLabel = option.label;
        this.rows.update((rows) => [...rows]);
      },
      this.openTransferMenu.bind(this),
      'transfer',
    );
  }

  rowValidation(row: DraftTransactionRow): RowValidation {
    if (this.isEmpty(row)) {
      return { valid: true, errors: [] };
    }

    const errors: string[] = [];
    if (!row.description.trim()) errors.push(this.messages.validation.descriptionRequired);
    if (!row.amount.trim()) errors.push(this.messages.validation.amountRequired);
    if (!row.type) errors.push(this.messages.validation.typeRequired);
    if (!isBrazilianDate(normalizeDraftDate(row.date)))
      errors.push(this.messages.validation.dateFormat);
    if (!this.matchedCategory(row)) errors.push(this.messages.validation.invalidCategory);
    if (!row.accountCode) errors.push(this.messages.validation.accountRequired);
    if (row.type === 'TRANSFER') {
      if (!row.transferAccountCode) errors.push(this.messages.validation.transferAccountRequired);
      if (row.accountCode && row.accountCode === row.transferAccountCode)
        errors.push(this.messages.validation.transferAccountsDifferent);
    }

    return { valid: errors.length === 0, errors };
  }

  canSubmit(): boolean {
    const filledRows = this.filledRows();
    return filledRows.length > 0 && filledRows.every((row) => this.rowValidation(row).valid);
  }

  openNotesModal(row: DraftTransactionRow): void {
    this.closeAllMenus();
    this.notesModalRowId.set(row.id);
    this.notesDraft.set(row.notes);
  }

  closeNotesModal(): void {
    this.notesModalRowId.set(null);
    this.notesDraft.set('');
  }

  saveNotes(): void {
    const rowId = this.notesModalRowId();
    if (rowId === null) {
      return;
    }

    const row = this.rows().find((candidate) => candidate.id === rowId);
    if (!row) {
      this.closeNotesModal();
      return;
    }

    row.notes = this.notesDraft();
    this.rows.update((rows) => [...rows]);
    this.closeNotesModal();
  }

  submit(): void {
    if (!this.canSubmit()) {
      return;
    }
    const payload = this.filledRows().map((row) => this.toPayload(row));
    this.saving.set(true);

    this.transactionsService.createMany(payload).subscribe({
      next: () => {
        this.referenceData.reload().subscribe({
          error: (error) => this.toast.error(getApiErrorMessage(error)),
        });
        this.toast.success(insertTransactionsSuccessMessage(payload.length));
        this.resetRows();
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.saving.set(false);
      },
      complete: () => this.saving.set(false),
    });
  }

  accountPreviewRows(): AccountPreviewRow[] {
    const accountsByCode = new Map(
      this.referenceData.accounts().map((account) => [account.Code, account]),
    );
    const impacts = new Map<string, number>();

    for (const row of this.filledRows()) {
      const amount = decimalToCents(row.amount);
      if (!row.accountCode || amount <= 0) {
        continue;
      }

      if (row.type === 'TRANSFER') {
        impacts.set(row.accountCode, (impacts.get(row.accountCode) ?? 0) - amount);
        if (row.transferAccountCode) {
          impacts.set(
            row.transferAccountCode,
            (impacts.get(row.transferAccountCode) ?? 0) + amount,
          );
        }
        continue;
      }

      const signedAmount = row.type === 'EXPENSE' ? -amount : amount;
      impacts.set(row.accountCode, (impacts.get(row.accountCode) ?? 0) + signedAmount);
    }

    return Array.from(impacts.entries())
      .map(([code, draftImpact]) => {
        const account = accountsByCode.get(code);
        const currentBalance = account?.Balance ?? 0;
        return {
          code,
          name: account?.Name ?? code,
          currentBalance,
          draftImpact,
          projectedBalance: currentBalance + draftImpact,
        };
      })
      .sort((left, right) => left.name.localeCompare(right.name, 'pt-BR'));
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrency(value);
  }

  private filledRows(): DraftTransactionRow[] {
    return this.rows().filter((row) => !this.isEmpty(row));
  }

  private toPayload(row: DraftTransactionRow): TransactionPayload {
    const amount = decimalToCents(row.amount);
    const normalizedDate = normalizeDraftDate(row.date);
    return {
      date: dateInputToIso(brazilianDateToQuery(normalizedDate)),
      description: row.description.trim(),
      notes: row.notes.trim() ? row.notes : null,
      amount: row.type === 'EXPENSE' ? -amount : amount,
      account_code: row.accountCode,
      category_code: this.matchedCategory(row)!.Code,
      is_transfer: row.type === 'TRANSFER',
      account_transfer: row.type === 'TRANSFER' ? row.transferAccountCode : null,
      exclude_from_dashboard: row.type === 'TRANSFER' ? false : row.excludeFromDashboard,
    };
  }

  private matchedCategory(row: DraftTransactionRow): Category | undefined {
    const normalized = normalize(row.category);
    if (!normalized) {
      return undefined;
    }

    const options = this.suggestedCategoryOptions(row);
    const exactMatch = options.find((category) => normalize(category.Name) === normalized);
    if (exactMatch) {
      return exactMatch;
    }

    const prefixMatches = options.filter((category) =>
      normalize(category.Name).startsWith(normalized),
    );
    if (prefixMatches.length > 0) {
      return prefixMatches[0];
    }

    const containsMatches = options.filter((category) =>
      normalize(category.Name).includes(normalized),
    );
    if (containsMatches.length > 0) {
      return containsMatches[0];
    }

    return undefined;
  }

  private matchedTypeOption(row: DraftTransactionRow): PickerOption | undefined {
    return this.matchPickerOption(this.typeOptions(), row.typeLabel);
  }

  private matchedAccountOption(row: DraftTransactionRow): PickerOption | undefined {
    return this.matchPickerOption(this.accountOptions(row), row.accountLabel);
  }

  private matchedTransferOption(row: DraftTransactionRow): PickerOption | undefined {
    return this.matchPickerOption(this.transferOptions(row), row.transferAccountLabel);
  }

  private resolveCategoryLabel(row: DraftTransactionRow): void {
    const match = this.matchedCategory(row);
    if (match) {
      row.category = match.Name;
      this.rows.update((rows) => [...rows]);
    }
  }

  finishCategoryEdit(row: DraftTransactionRow): void {
    this.closeCategoryMenuTimer = setTimeout(() => {
      if (this.activeMenuKind() === 'category' && this.categoryMenuRowId() === row.id) {
        this.categoryMenuRowId.set(null);
        this.activeMenuKind.set(null);
      }
      this.closeCategoryMenuTimer = null;
    }, 120);
    this.resolveCategoryLabel(row);
  }

  finishTypeEdit(row: DraftTransactionRow): void {
    this.closeTypeMenuTimer = setTimeout(() => {
      if (this.activeMenuKind() === 'type' && this.typeMenuRowId() === row.id) {
        this.typeMenuRowId.set(null);
        this.activeMenuKind.set(null);
      }
      this.closeTypeMenuTimer = null;
    }, 120);
    const match = this.matchedTypeOption(row);
    this.setRowType((row), (match?.value as EntryType) ?? '', match?.label ?? row.typeLabel);
    this.rows.update((rows) => [...rows]);
  }

  finishAccountEdit(row: DraftTransactionRow): void {
    this.closeAccountMenuTimer = setTimeout(() => {
      if (this.activeMenuKind() === 'account' && this.accountMenuRowId() === row.id) {
        this.accountMenuRowId.set(null);
        this.activeMenuKind.set(null);
      }
      this.closeAccountMenuTimer = null;
    }, 120);
    const match = this.matchedAccountOption(row);
    row.accountCode = match?.value ?? '';
    row.accountLabel = match?.label ?? row.accountLabel;
    this.rows.update((rows) => [...rows]);
  }

  finishTransferEdit(row: DraftTransactionRow): void {
    this.closeTransferMenuTimer = setTimeout(() => {
      if (this.activeMenuKind() === 'transfer' && this.transferMenuRowId() === row.id) {
        this.transferMenuRowId.set(null);
        this.activeMenuKind.set(null);
      }
      this.closeTransferMenuTimer = null;
    }, 120);
    const match = this.matchedTransferOption(row);
    row.transferAccountCode = match?.value ?? '';
    row.transferAccountLabel = match?.label ?? row.transferAccountLabel;
    this.rows.update((rows) => [...rows]);
  }

  private leafCategories(): Category[] {
    const parentCodes = new Set(
      this.referenceData
        .activeCategories()
        .filter((category) => (category.SubCategories?.length ?? 0) > 0)
        .map((category) => category.Code),
    );
    return this.referenceData
      .activeFlatCategories()
      .filter((category) => !parentCodes.has(category.Code));
  }

  private ensureRowCount(requiredCount: number): void {
    const missing = requiredCount - this.rows().length;
    if (missing <= 0) {
      return;
    }
    this.addRows(missing);
  }

  private closeAllMenus(): void {
    this.cancelCategoryMenuClose();
    this.cancelTypeMenuClose();
    this.cancelAccountMenuClose();
    this.cancelTransferMenuClose();
    this.categoryMenuRowId.set(null);
    this.typeMenuRowId.set(null);
    this.accountMenuRowId.set(null);
    this.transferMenuRowId.set(null);
    this.activeMenuKind.set(null);
  }

  private pasteRows(text: string): string[][] {
    return text
      .replace(/\r\n/g, '\n')
      .replace(/\r/g, '\n')
      .replace(/\n$/, '')
      .split('\n')
      .map((line) => line.split('\t'));
  }

  private applyPastedValue(row: DraftTransactionRow, columnIndex: number, rawValue: string): void {
    const value = rawValue.trim();
    switch (columnIndex) {
      case 0:
        row.description = value;
        this.onDescriptionChange(row);
        break;
      case 1:
        row.amount = normalizeDraftAmount(value);
        row.amountManualDecimal = value.includes(',');
        break;
      case 2:
        row.date = normalizeDraftDate(value);
        break;
      case 3:
        row.typeLabel = value;
        break;
      case 4:
        row.category = value;
        break;
      case 5:
        row.accountLabel = value;
        break;
      case 6:
        row.transferAccountLabel = value;
        break;
      default:
        break;
    }
  }

  private normalizeRowAfterPaste(row: DraftTransactionRow): void {
    const categoryBeforeTypeSync = row.category;
    const typeMatch = this.matchEntryType(row.typeLabel);
    this.setRowType(
      row,
      (typeMatch?.value as EntryType | undefined) ?? row.type,
      typeMatch?.label ?? row.typeLabel,
    );
    row.category = categoryBeforeTypeSync;

    row.amount = normalizeDraftAmount(row.amount);
    row.amountManualDecimal = row.amount.includes(',');
    row.date = normalizeDraftDate(row.date);

    const accountMatch = this.matchedAccountOption(row);
    row.accountCode = accountMatch?.value ?? row.accountCode;
    row.accountLabel = accountMatch?.label ?? row.accountLabel;

    if (row.type === 'TRANSFER') {
      const transferMatch = this.matchedTransferOption(row);
      row.transferAccountCode = transferMatch?.value ?? row.transferAccountCode;
      row.transferAccountLabel = transferMatch?.label ?? row.transferAccountLabel;
    } else {
      row.transferAccountCode = '';
      row.transferAccountLabel = '';
    }

    this.resolveCategoryLabel(row);
  }

  private matchEntryType(rawValue: string): PickerOption | undefined {
    const normalized = normalize(rawValue);
    if (!normalized) {
      return undefined;
    }

    return this.typeOptions().find(
      (option) => normalize(option.label) === normalized || normalize(option.value) === normalized,
    );
  }

  private categoryPickerMenuLabel(category: Category): string {
    if (!category.ParentID) {
      return category.Name;
    }

    const parent = this.referenceData
      .activeFlatCategories()
      .find((candidate) => candidate.ID === category.ParentID);
    return parent ? `${parent.Name} - ${category.Name}` : category.Name;
  }

  private resetRows(): void {
    this.rows.set(Array.from({ length: INITIAL_ROWS }, () => this.createEmptyRow()));
  }

  private focusFirstDescriptionInput(): void {
    setTimeout(() => {
      const input = this.elementRef.nativeElement.querySelector<HTMLInputElement>(
        'tbody tr:first-child td:first-child input.grid-input',
      );
      input?.focus();
    }, 0);
  }

  private createEmptyRow(id = this.nextId++): DraftTransactionRow {
    return {
      id,
      description: '',
      notes: '',
      amount: '',
      amountManualDecimal: false,
      type: '',
      typeLabel: '',
      date: '',
      dateAutoFilled: false,
      dateManuallyEdited: false,
      category: '',
      accountCode: '',
      accountLabel: '',
      transferAccountCode: '',
      transferAccountLabel: '',
      excludeFromDashboard: false,
    };
  }

  private isEmpty(row: DraftTransactionRow): boolean {
    return (
      !row.description.trim() &&
      !row.notes.trim() &&
      !row.amount.trim() &&
      !row.type &&
      !row.typeLabel.trim() &&
      !row.date &&
      !row.category.trim() &&
      !row.accountCode &&
      !row.accountLabel.trim() &&
      !row.transferAccountCode &&
      !row.transferAccountLabel.trim()
    );
  }

  isAnyMenuOpenForRow(rowId: number): boolean {
    return (
      this.categoryMenuRowId() === rowId ||
      this.typeMenuRowId() === rowId ||
      this.accountMenuRowId() === rowId ||
      this.transferMenuRowId() === rowId
    );
  }

  private scrollActiveCategoryOptionIntoView(rowId: number): void {
    this.scrollPickerOptionIntoView('category', rowId, this.categoryMenuActiveIndex());
  }

  private scrollPickerOptionIntoView(prefix: string, rowId: number, index: number): void {
    void prefix;
    void rowId;
    const activeOption = this.elementRef.nativeElement.querySelector<HTMLElement>(
      `.shell-menu [data-menu-option="${index}"]`,
    );
    activeOption?.scrollIntoView({ block: 'nearest' });
  }

  private updateMenuDirection(menuAnchor: string): void {
    const input = this.elementRef.nativeElement.querySelector<HTMLElement>(
      `[data-menu-anchor="${menuAnchor}"]`,
    );
    if (!input) {
      return;
    }

    const rect = input.getBoundingClientRect();
    const shell = input.closest('.insert-table-shell') as HTMLElement | null;
    const shellRect = shell?.getBoundingClientRect();
    const menuHeight = 220;
    const lowerBoundary = shellRect?.bottom ?? window.innerHeight;
    const upperBoundary = shellRect?.top ?? 0;
    const spaceBelow = lowerBoundary - rect.bottom;
    const spaceAbove = rect.top - upperBoundary;
    const openUpward = spaceBelow < 160 && spaceAbove > spaceBelow && spaceAbove > 80;
    this.menuOpenUpward.set(openUpward && rect.top > menuHeight + 12);
    if (shellRect) {
      const top = this.menuOpenUpward()
        ? rect.top - shellRect.top - menuHeight - 2
        : rect.bottom - shellRect.top + 2;
      const maxLeft = Math.max(0, shellRect.width - rect.width);
      this.menuPosition.set({
        top: Math.max(0, top),
        left: Math.max(0, Math.min(rect.left - shellRect.left, maxLeft)),
        width: rect.width,
      });
    }
  }

  private filterPickerOptions(options: PickerOption[], rawValue: string): PickerOption[] {
    const normalized = normalize(rawValue);
    if (!normalized) {
      return options;
    }
    const startsWith = options.filter((option) => normalize(option.label).startsWith(normalized));
    if (startsWith.length > 0) {
      return startsWith;
    }
    const includes = options.filter((option) => normalize(option.label).includes(normalized));
    return includes.length > 0 ? includes : options;
  }

  private matchPickerOption(options: PickerOption[], rawValue: string): PickerOption | undefined {
    const normalized = normalize(rawValue);
    if (!normalized) {
      return undefined;
    }
    const exact = options.find((option) => normalize(option.label) === normalized);
    if (exact) {
      return exact;
    }
    const startsWith = options.filter((option) => normalize(option.label).startsWith(normalized));
    if (startsWith.length > 0) {
      return startsWith[0];
    }
    const includes = options.filter((option) => normalize(option.label).includes(normalized));
    if (includes.length > 0) {
      return includes[0];
    }
    return undefined;
  }

  private handlePickerKeydown(
    event: KeyboardEvent,
    rowId: number,
    options: PickerOption[],
    rowSignal: { (): number | null; set(value: number | null): void },
    indexSignal: {
      (): number;
      set(value: number): void;
      update(updater: (value: number) => number): void;
    },
    onSelect: (option: PickerOption) => void,
    openMenu: (rowId: number) => void,
    prefix: string,
  ): void {
    if (options.length === 0) {
      return;
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      openMenu(rowId);
      indexSignal.update((index) => (index + 1) % options.length);
      queueMicrotask(() => this.scrollPickerOptionIntoView(prefix, rowId, indexSignal()));
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      openMenu(rowId);
      indexSignal.update((index) => (index - 1 + options.length) % options.length);
      queueMicrotask(() => this.scrollPickerOptionIntoView(prefix, rowId, indexSignal()));
      return;
    }
    if (event.key === 'Enter' && rowSignal() === rowId) {
      event.preventDefault();
      const option = options[indexSignal()] ?? options[0];
      if (option) {
        onSelect(option);
        rowSignal.set(null);
        this.activeMenuKind.set(null);
      }
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      rowSignal.set(null);
      this.activeMenuKind.set(null);
    }
  }

  private activeMenuRow(): DraftTransactionRow | undefined {
    const rowId =
      this.categoryMenuRowId() ??
      this.typeMenuRowId() ??
      this.accountMenuRowId() ??
      this.transferMenuRowId();
    return this.rows().find((row) => row.id === rowId);
  }

  private applySuggestionToRow(row: DraftTransactionRow): void {
    const suggestion = this.bestMatchingSuggestion(row.description);
    if (!suggestion) {
      return;
    }

    if (suggestion.entry_type) {
      const nextType = this.rowTypeFromSuggestion(suggestion.entry_type);
      const option = this.typeOptions().find((candidate) => candidate.value === nextType);
      if (option) {
        this.setRowType(row, nextType, option.label);
      }
    }

    if (suggestion.category_code) {
      const category = this.referenceData
        .flatCategories()
        .find((candidate) => candidate.Code === suggestion.category_code);
      if (category) {
        row.category = category.Name;
      }
    }

    if (suggestion.account_code) {
      const account = this.referenceData
        .accounts()
        .find((candidate) => candidate.Code === suggestion.account_code);
      if (account) {
        row.accountCode = account.Code;
        row.accountLabel = account.Name;
      }
    }

    if (suggestion.transfer_account_code) {
      const transferAccount = this.referenceData
        .accounts()
        .find((candidate) => candidate.Code === suggestion.transfer_account_code);
      if (transferAccount) {
        row.transferAccountCode = transferAccount.Code;
        row.transferAccountLabel = transferAccount.Name;
      }
    }
  }

  private bestMatchingSuggestion(description: string): Suggestion | undefined {
    const normalizedDescription = normalize(description);
    if (!normalizedDescription) {
      return undefined;
    }

    return this.referenceData
      .suggestions()
      .map((suggestion, index) => ({ suggestion, index }))
      .filter(({ suggestion }) => {
        const matchText = normalize(suggestion.description_contains);
        return matchText && normalizedDescription.includes(matchText);
      })
      .sort((left, right) => {
        if (left.suggestion.priority !== right.suggestion.priority) {
          return left.suggestion.priority - right.suggestion.priority;
        }

        const lengthDiff =
          right.suggestion.description_contains.length -
          left.suggestion.description_contains.length;
        if (lengthDiff !== 0) {
          return lengthDiff;
        }

        return left.index - right.index;
      })[0]?.suggestion;
  }

  private rowTypeFromSuggestion(entryType: SuggestionEntryType): EntryType {
    switch (entryType) {
      case 'REVENUE':
        return 'REVENUE';
      case 'EXPENSE':
        return 'EXPENSE';
      case 'TRANSFER':
        return 'TRANSFER';
    }
  }

  private setRowType(row: DraftTransactionRow, nextType: EntryType, nextLabel: string): void {
    const previousType = row.type;
    row.type = nextType;
    row.typeLabel = nextLabel;

    if (previousType !== nextType) {
      if (nextType === 'TRANSFER') {
        row.excludeFromDashboard = false;
      }
      this.onTypeChange(row);
    }
  }

  handleAmountKeydown(event: KeyboardEvent, row: DraftTransactionRow): void {
    if (event.key === ',') {
      row.amountManualDecimal = true;
    }
  }
}

function normalize(value: string): string {
  return value
    .trim()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLocaleLowerCase('pt-BR');
}

function todayBrazilianDate(): string {
  const today = new Date();
  const day = String(today.getDate()).padStart(2, '0');
  const month = String(today.getMonth() + 1).padStart(2, '0');
  return `${day}/${month}/${today.getFullYear()}`;
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

function compactBrazilianDate(value: string): string {
  const [day, month, year] = value.split('/').map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  const monthLabel = new Intl.DateTimeFormat('pt-BR', {
    month: 'short',
    timeZone: 'UTC',
  }).format(date);

  return `${String(day).padStart(2, '0')}-${monthLabel}`;
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

function isBrazilianDate(value: string): boolean {
  if (!/^\d{2}\/\d{2}\/\d{4}$/.test(value)) {
    return false;
  }
  const [day, month, year] = value.split('/').map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day
  );
}
