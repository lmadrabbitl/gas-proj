import { Component, DestroyRef, ElementRef, HostListener, OnInit, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { forkJoin, of, switchMap } from 'rxjs';

import { InvestmentsService } from '../../data/investments.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { investmentAssetLabel, investmentAssetTypeLabel, investmentOperationTypeLabel } from '../../shared/labels';
import { uiMessages } from '../../shared/messages';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { brazilianDateToQuery, centsToDecimal, dateInputToIso, decimalToCents, toBrazilianDate, toDateInputValue } from '../../shared/money';
import { InvestmentAsset, InvestmentAssetType, InvestmentOperation, InvestmentOperationType } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

type OperationsFilterMenuType = 'asset' | 'operation';

@Component({
  selector: 'app-investment-operations',
  imports: [ReactiveFormsModule, RouterLink, RouterLinkActive],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.eyebrow }}</p>
        <h1>{{ messages.title }}</h1>
        <p class="page-subtitle">{{ messages.subtitle }}</p>
      </div>
    </section>

    <nav class="panel investment-subnav">
      <a routerLink="/investments/positions" routerLinkActive="active">{{ nav.positions }}</a>
      <a routerLink="/investments/assets" routerLinkActive="active">{{ nav.assets }}</a>
      <a routerLink="/investments/insert" routerLinkActive="active">{{ nav.insert }}</a>
      <a routerLink="/investments/operations" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }">
        {{ nav.operations }}
      </a>
      <a routerLink="/investments/portfolios" routerLinkActive="active">{{ nav.portfolios }}</a>
    </nav>

    <section class="panel">
      <form class="filters operations-filters" [formGroup]="filters">
        <div class="filter-field">
          <span class="filter-field-label">{{ messages.filters.asset }}</span>
          <div class="multi-select" data-multi-select="asset">
            <button class="multi-select-trigger" type="button" (click)="toggleFilterMenu('asset')">
              <span class="multi-select-value">{{ selectedAssetLabel() }}</span>
              <span class="multi-select-caret" aria-hidden="true">{{ assetMenuOpen() ? '▴' : '▾' }}</span>
            </button>
            @if (assetMenuOpen()) {
              <div class="multi-select-menu">
                @for (assetCode of assetFilterOptions(); track assetCode) {
                  <label class="multi-select-option">
                    <input
                      type="checkbox"
                      [checked]="isFilterSelected('asset', assetCode)"
                      (change)="toggleFilterSelection('asset', assetCode)"
                    />
                    <span>{{ assetCode }}</span>
                  </label>
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
                @for (operationTypeValue of operationFilterOptions; track operationTypeValue) {
                  <label class="multi-select-option">
                    <input
                      type="checkbox"
                      [checked]="isFilterSelected('operation', operationTypeValue)"
                      (change)="toggleFilterSelection('operation', operationTypeValue)"
                    />
                    <span>{{ operationType(operationTypeValue) }}</span>
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
        <p class="state-message">{{ messages.loading }}</p>
      } @else if (operations().length === 0) {
        <p class="state-message">{{ messages.empty }}</p>
      } @else if (filteredOperations().length === 0) {
        <p class="state-message">{{ messages.filteredEmpty }}</p>
      } @else {
        <div class="table-wrap">
          <table class="operations-table">
            <colgroup>
              <col class="operations-col-date" />
              <col class="operations-col-asset" />
              <col class="operations-col-type" />
              <col class="operations-col-quantity" />
              <col class="operations-col-unit-price" />
              <col class="operations-col-fee" />
              <col class="operations-col-net" />
              <col class="operations-col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>{{ messages.columns.date }}</th>
                <th>{{ messages.columns.asset }}</th>
                <th>{{ messages.columns.type }}</th>
                <th>{{ messages.columns.quantity }}</th>
                <th>{{ messages.columns.unitPrice }}</th>
                <th>{{ messages.columns.fee }}</th>
                <th>{{ messages.columns.net }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              @for (operation of filteredOperations(); track operation.id) {
                <tr>
                  <td>{{ brazilianDate(operation.date) }}</td>
                  <td>{{ operation.asset_code }}</td>
                  <td>{{ operationType(operation.operation_type) }}</td>
                  <td>{{ operation.quantity }}</td>
                  <td>{{ money(operation.unit_price) }}</td>
                  <td>{{ money(operation.fee_amount) }}</td>
                  <td>{{ money(operation.net_amount) }}</td>
                  <td class="actions-cell operations-actions-cell">
                    <div class="operations-actions">
                      <button
                        class="icon-action note-info-action"
                        type="button"
                        [attr.title]="notesTooltip(operation)"
                        [attr.aria-label]="notesTooltip(operation)"
                      >
                        i
                      </button>
                      <button class="icon-action" type="button" [title]="messages.actions.edit" [attr.aria-label]="messages.actions.edit" (click)="openEdit(operation)">
                        <svg aria-hidden="true" viewBox="0 0 24 24">
                          <path
                            d="M4 20h4.5L19 9.5a1.5 1.5 0 0 0 0-2.12l-2.38-2.38A1.5 1.5 0 0 0 14.5 5L4 15.5V20Zm0 0 4.5-4.5M12.5 7.5l4 4"
                            fill="none"
                            stroke="currentColor"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.8"
                          />
                        </svg>
                      </button>
                      <button class="icon-action danger" type="button" [title]="messages.actions.remove" [attr.aria-label]="messages.actions.remove" (click)="remove(operation)">
                        <svg aria-hidden="true" viewBox="0 0 24 24">
                          <path d="M9 3h6l1 2h4v2H4V5h4l1-2Zm-1 6h2v10H8V9Zm6 0h2v10h-2V9Zm-9 0h14l-1 12H6L5 9Z"/>
                        </svg>
                      </button>
                    </div>
                  </td>
                </tr>
              }
            </tbody>
          </table>
        </div>
      }
    </section>

    @if (panelOpen()) {
      <aside class="side-panel">
        <div class="panel-header">
          <h2>{{ editing() ? messages.form.editTitle : messages.form.createTitle }}</h2>
          <button class="ghost-button" type="button" (click)="closePanel()">{{ messages.actions.close }}</button>
        </div>
        <form class="form-stack" [formGroup]="form" (ngSubmit)="save()">
          <label>
            {{ messages.form.asset }}
            <select formControlName="asset_code">
              <option value="">Selecione</option>
                  @for (asset of activeAssets(); track asset.code) {
                    <option [value]="asset.code">{{ assetLabel(asset.code, asset.name) }}</option>
              }
            </select>
          </label>

          <button class="ghost-button" type="button" (click)="toggleAssetMode()">
            {{ assetCreateMode() ? messages.actions.cancelAsset : messages.actions.createAsset }}
          </button>

          @if (assetCreateMode()) {
            <div class="inline-box">
              <strong>{{ messages.form.createAssetTitle }}</strong>
              <label>
                {{ messages.form.assetCode }}
                <input formControlName="new_asset_code" />
              </label>
              <label>
                {{ messages.form.assetName }}
                <input formControlName="new_asset_name" />
              </label>
              <label>
                {{ messages.form.assetType }}
                <select formControlName="new_asset_type">
                  <option value="STOCK">{{ assetType('STOCK') }}</option>
                  <option value="FII">{{ assetType('FII') }}</option>
                  <option value="ETF">{{ assetType('ETF') }}</option>
                </select>
              </label>
            </div>
          }

          <label>
            {{ messages.form.operationType }}
            <select formControlName="operation_type">
              <option value="BUY">{{ operationType('BUY') }}</option>
              <option value="SELL">{{ operationType('SELL') }}</option>
              <option value="BONIFICATION">{{ operationType('BONIFICATION') }}</option>
            </select>
          </label>
          <label>
            {{ messages.form.quantity }}
            <input type="number" min="1" formControlName="quantity" />
          </label>
          <label>
            {{ messages.form.unitPrice }}
            <input formControlName="unit_price" inputmode="decimal" />
          </label>
          <label>
            {{ messages.form.feeAmount }}
            <input formControlName="fee_amount" inputmode="decimal" />
          </label>
          <label>
            {{ messages.form.date }}
            <input type="date" formControlName="date" />
          </label>
          <label>
            {{ messages.form.notes }}
            <textarea rows="3" formControlName="notes"></textarea>
          </label>
          <button class="primary-button" type="submit" [disabled]="saving() || form.invalid">
            {{ saving() ? messages.actions.saving : messages.actions.save }}
          </button>
        </form>
      </aside>
    }
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
    }
    .investment-subnav a.active {
      color: var(--text);
      background: var(--accent-soft);
    }
    .page-subtitle {
      margin: 6px 0 0;
      color: var(--muted);
    }

    .operations-filters {
      grid-template-columns: repeat(2, minmax(180px, 1fr)) repeat(2, minmax(96px, 0.7fr));
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
      max-height: 360px;
      min-width: max(100%, 220px);
      max-width: min(420px, calc(100vw - 32px));
      overflow-x: auto;
      overflow-y: auto;
      padding: 10px;
      position: absolute;
      scrollbar-color: #b7c7c0 transparent;
      scrollbar-width: thin;
      top: 100%;
      z-index: 20;
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

    .operations-table {
      width: 100%;
      table-layout: fixed;
    }

    .operations-table th,
    .operations-table td {
      white-space: nowrap;
    }

    .operations-table tbody tr:nth-child(even) td {
      background: var(--surface-soft);
    }

    .operations-col-date {
      width: 118px;
    }

    .operations-col-asset {
      width: 88px;
    }

    .operations-col-type {
      width: 88px;
    }

    .operations-col-quantity {
      width: 82px;
    }

    .operations-col-unit-price,
    .operations-col-fee {
      width: 102px;
    }

    .operations-col-net {
      width: 126px;
    }

    .operations-col-actions {
      width: 96px;
    }

    .inline-box {
      display: grid;
      gap: 12px;
      padding: 14px;
      border: 1px solid var(--border);
      border-radius: 16px;
      background: var(--surface-soft);
    }

    .note-info-action {
      font-size: 0.9rem;
      font-weight: 700;
    }

    .operations-actions-cell {
      padding-left: 6px;
      padding-right: 6px;
    }

    .operations-actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 6px;
    }

    .operations-actions .icon-action {
      margin-left: 0;
    }
  `],
})
export class InvestmentOperationsComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly destroyRef = inject(DestroyRef);
  private readonly elementRef = inject(ElementRef<HTMLElement>);
  private readonly moneyVisibility = inject(MoneyVisibilityService);
  readonly nav = uiMessages.investments.nav;
  readonly messages = uiMessages.investments.operations;
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly panelOpen = signal(false);
  readonly assetCreateMode = signal(false);
  readonly assetMenuOpen = signal(false);
  readonly operationMenuOpen = signal(false);
  readonly operations = signal<InvestmentOperation[]>([]);
  readonly assets = signal<InvestmentAsset[]>([]);
  readonly editing = signal<InvestmentOperation | null>(null);
  readonly activeAssets = computed(() => this.assets().filter((asset) => asset.is_active));
  readonly operationFilterOptions: InvestmentOperationType[] = ['BUY', 'SELL', 'BONIFICATION'];
  readonly assetFilterOptions = computed(() => {
    const codes = new Set(this.operations().map((operation) => operation.asset_code));
    return Array.from(codes).sort((left, right) => left.localeCompare(right));
  });
  readonly filtersValue = signal({
    asset_codes: [] as string[],
    operation_types: [] as InvestmentOperationType[],
    from_date: '',
    to_date: '',
  });
  readonly filteredOperations = computed(() => {
    const filters = this.filtersValue();
    const assetCodes = new Set(filters.asset_codes);
    const operationTypes = new Set(filters.operation_types);
    const fromDate = brazilianDateToQuery(filters.from_date);
    const toDate = brazilianDateToQuery(filters.to_date);

    return this.operations().filter((operation) => {
      const operationDate = operation.date.slice(0, 10);
      if (assetCodes.size > 0 && !assetCodes.has(operation.asset_code)) {
        return false;
      }
      if (operationTypes.size > 0 && !operationTypes.has(operation.operation_type)) {
        return false;
      }
      if (fromDate && operationDate < fromDate) {
        return false;
      }
      if (toDate && operationDate > toDate) {
        return false;
      }
      return true;
    });
  });
  readonly filters = this.fb.nonNullable.group({
    asset_codes: this.fb.nonNullable.control<string[]>([]),
    operation_types: this.fb.nonNullable.control<InvestmentOperationType[]>([]),
    from_date: [''],
    to_date: [''],
  });
  readonly form = this.fb.group({
    asset_code: this.fb.nonNullable.control('', Validators.required),
    operation_type: this.fb.nonNullable.control<InvestmentOperationType>('BUY', Validators.required),
    quantity: this.fb.nonNullable.control(1, Validators.required),
    unit_price: this.fb.nonNullable.control('', Validators.required),
    fee_amount: this.fb.nonNullable.control('0,00'),
    date: this.fb.nonNullable.control('', Validators.required),
    notes: this.fb.nonNullable.control(''),
    new_asset_code: this.fb.nonNullable.control(''),
    new_asset_name: this.fb.nonNullable.control(''),
    new_asset_type: this.fb.nonNullable.control<InvestmentAssetType>('STOCK'),
  });

  constructor(
    private readonly investmentsService: InvestmentsService,
    private readonly toast: ToastService,
  ) {
    this.filters.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((value) => {
        this.filtersValue.set({
          asset_codes: value.asset_codes ?? [],
          operation_types: value.operation_types ?? [],
          from_date: value.from_date ?? '',
          to_date: value.to_date ?? '',
        });
      });

    this.filters.controls.from_date.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((value) => this.normalizeFilterDateControl('from_date', value));

    this.filters.controls.to_date.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((value) => this.normalizeFilterDateControl('to_date', value));
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent): void {
    const target = event.target as HTMLElement | null;
    if (target?.closest('[data-multi-select="asset"]')) {
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
    this.syncAssetValidators();
    this.load();
  }

  load(): void {
    this.loading.set(true);
    forkJoin({
      operations: this.investmentsService.listOperations(),
      assets: this.investmentsService.listAssets(),
    }).subscribe({
      next: ({ operations, assets }) => {
        this.operations.set(operations);
        this.assets.set(assets);
        this.loading.set(false);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
    });
  }

  openCreate(): void {
    this.editing.set(null);
    this.assetCreateMode.set(false);
    this.form.reset({
      asset_code: '',
      operation_type: 'BUY',
      quantity: 1,
      unit_price: '',
      fee_amount: '0,00',
      date: toDateInputValue(new Date()),
      notes: '',
      new_asset_code: '',
      new_asset_name: '',
      new_asset_type: 'STOCK',
    });
    this.syncAssetValidators();
    this.panelOpen.set(true);
  }

  openEdit(operation: InvestmentOperation): void {
    this.editing.set(operation);
    this.assetCreateMode.set(false);
    this.form.reset({
      asset_code: operation.asset_code,
      operation_type: operation.operation_type,
      quantity: operation.quantity,
      unit_price: this.moneyInputValue(operation.unit_price),
      fee_amount: this.moneyInputValue(operation.fee_amount),
      date: toDateInputValue(operation.date),
      notes: operation.notes ?? '',
      new_asset_code: '',
      new_asset_name: '',
      new_asset_type: operation.asset_type,
    });
    this.syncAssetValidators();
    this.panelOpen.set(true);
  }

  closePanel(): void {
    this.panelOpen.set(false);
    this.editing.set(null);
  }

  toggleAssetMode(): void {
    this.assetCreateMode.update((value) => !value);
    if (this.assetCreateMode()) {
      this.form.controls.asset_code.setValue('');
    }
    this.syncAssetValidators();
  }

  save(): void {
    if (this.form.invalid) {
      return;
    }
    this.saving.set(true);
    const value = this.form.getRawValue();
    const operationPayload = {
      asset_code: value.asset_code,
      operation_type: value.operation_type,
      quantity: Number(value.quantity),
      unit_price: decimalToCents(value.unit_price),
      fee_amount: decimalToCents(value.fee_amount),
      date: dateInputToIso(value.date),
      notes: value.notes,
    };
    const createAsset$ = this.assetCreateMode()
      ? this.investmentsService.createAsset({
          code: value.new_asset_code,
          name: value.new_asset_name,
          asset_type: value.new_asset_type,
        })
      : of<InvestmentAsset | null>(null);

    createAsset$
      .pipe(
        switchMap((asset) => {
          const assetCode = asset?.code ?? operationPayload.asset_code;
          if (this.editing()) {
            return this.investmentsService.updateOperation(this.editing()!.id, {
              ...operationPayload,
              asset_code: assetCode,
            });
          }
          return this.investmentsService.createOperation({
            ...operationPayload,
            asset_code: assetCode,
          });
        }),
      )
      .subscribe({
        next: () => {
          this.toast.success('Operação salva.');
          this.closePanel();
          this.load();
        },
        error: (error: unknown) => {
          this.toast.error(getApiErrorMessage(error));
          this.saving.set(false);
        },
        complete: () => this.saving.set(false),
      });
  }

  remove(operation: InvestmentOperation): void {
    if (!window.confirm(`Excluir a operação ${operation.asset_code} de ${operation.quantity} unidade(s)?`)) {
      return;
    }
    this.investmentsService.deleteOperation(operation.id).subscribe({
      next: () => {
        this.toast.success('Operação excluída.');
        this.load();
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  operationType(type: InvestmentOperationType): string {
    return investmentOperationTypeLabel(type);
  }

  toggleFilterMenu(type: OperationsFilterMenuType): void {
    if (type === 'asset') {
      this.assetMenuOpen.update((value) => !value);
      this.operationMenuOpen.set(false);
      return;
    }
    this.operationMenuOpen.update((value) => !value);
    this.assetMenuOpen.set(false);
  }

  toggleFilterSelection(type: OperationsFilterMenuType, value: string): void {
    const control = type === 'asset' ? this.filters.controls.asset_codes : this.filters.controls.operation_types;
    const currentValues = [...control.value];
    const nextValues = currentValues.includes(value)
      ? currentValues.filter((item) => item !== value)
      : [...currentValues, value];
    control.setValue(nextValues as never);
  }

  isFilterSelected(type: OperationsFilterMenuType, value: string): boolean {
    return (type === 'asset' ? this.filters.controls.asset_codes.value : this.filters.controls.operation_types.value).includes(value as never);
  }

  selectedAssetLabel(): string {
    return this.selectionLabel(this.filters.controls.asset_codes.value, this.messages.filters.allAssets);
  }

  selectedOperationLabel(): string {
    const selected = this.filters.controls.operation_types.value;
    if (selected.length === 0) {
      return this.messages.filters.allOperations;
    }
    if (selected.length === 1) {
      return this.operationType(selected[0]);
    }
    return `${selected.length} selecionadas`;
  }

  assetType(type: InvestmentAssetType): string {
    return investmentAssetTypeLabel(type);
  }

  assetLabel(code: string, name: string): string {
    return investmentAssetLabel(code, name);
  }

  hasNotes(operation: InvestmentOperation): boolean {
    return operation.notes.trim().length > 0;
  }

  notesTooltip(operation: InvestmentOperation): string {
    return this.hasNotes(operation) ? operation.notes : 'Sem observações';
  }

  private closeFilterMenus(): void {
    this.assetMenuOpen.set(false);
    this.operationMenuOpen.set(false);
  }

  private selectionLabel(values: string[], emptyLabel: string): string {
    if (values.length === 0) {
      return emptyLabel;
    }
    if (values.length === 1) {
      return values[0];
    }
    return `${values.length} selecionados`;
  }

  private normalizeFilterDateControl(controlName: 'from_date' | 'to_date', value: string): void {
    const digits = value.replace(/\D/g, '').slice(0, 8);
    let formatted = digits;
    if (digits.length > 4) {
      formatted = `${digits.slice(0, 2)}/${digits.slice(2, 4)}/${digits.slice(4)}`;
    } else if (digits.length > 2) {
      formatted = `${digits.slice(0, 2)}/${digits.slice(2)}`;
    }
    if (formatted === value) {
      return;
    }
    this.filters.controls[controlName].setValue(formatted, { emitEvent: false });
    const current = this.filtersValue();
    this.filtersValue.set({
      ...current,
      [controlName]: formatted,
    });
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrency(value);
  }

  moneyInputValue(value: number): string {
    return centsToDecimal(value).replace('.', ',');
  }

  brazilianDate(value: string): string {
    return toBrazilianDate(value);
  }

  private syncAssetValidators(): void {
    if (this.assetCreateMode()) {
      this.form.controls.asset_code.clearValidators();
      this.form.controls.new_asset_code.setValidators([Validators.required]);
      this.form.controls.new_asset_name.setValidators([Validators.required]);
    } else {
      this.form.controls.asset_code.setValidators([Validators.required]);
      this.form.controls.new_asset_code.clearValidators();
      this.form.controls.new_asset_name.clearValidators();
    }
    this.form.controls.asset_code.updateValueAndValidity({ emitEvent: false });
    this.form.controls.new_asset_code.updateValueAndValidity({ emitEvent: false });
    this.form.controls.new_asset_name.updateValueAndValidity({ emitEvent: false });
  }
}
