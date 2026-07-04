import { Component, DestroyRef, OnInit, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { startWith } from 'rxjs';

import { AccountsService } from '../../data/accounts.service';
import { ReferenceDataService } from '../../data/reference-data.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { accountTypeLabel } from '../../shared/labels';
import {
  uiMessages,
  deactivateAccountConfirmationMessage,
  deleteAccountPermanentConfirmationMessage,
} from '../../shared/messages';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { Account, AccountAssetRole, AccountType } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

@Component({
  selector: 'app-accounts',
  imports: [ReactiveFormsModule],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.page.eyebrow }}</p>
        <h1>{{ messages.page.title }}</h1>
      </div>
      <button class="primary-button" type="button" (click)="openCreate()">{{ messages.page.create }}</button>
    </section>
    <section class="panel">
      @if (loading()) {
        <p class="state-message">{{ messages.states.loading }}</p>
      } @else if (activeAccounts().length === 0 && inactiveAccounts().length === 0) {
        <p class="state-message">{{ messages.states.empty }}</p>
      } @else {
        @if (activeAccounts().length > 0) {
          <div class="table-section">
            <div class="table-section-header">
              <h2>{{ messages.sections.active }}</h2>
            </div>
            <div class="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th></th>
                    <th>{{ messages.columns.sortOrder }}</th>
                    <th>{{ messages.columns.name }}</th>
                    <th>{{ messages.columns.type }}</th>
                    <th>{{ messages.columns.assetRole }}</th>
                    <th>{{ messages.columns.dashboard }}</th>
                    <th>{{ messages.columns.balance }}</th>
                    <th>{{ messages.columns.currency }}</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  @for (account of activeAccounts(); track account.Code) {
                    <tr
                      [class.liability-row]="isLiability(account)"
                      [class.dragging-row]="draggingCode() === account.Code"
                      [class.drag-target-row]="dropTargetCode() === account.Code"
                      draggable="true"
                      (dragstart)="onDragStart($event, account)"
                      (dragover)="onDragOver($event, account)"
                      (drop)="onDrop($event, account)"
                      (dragend)="onDragEnd()"
                    >
                      <td class="drag-cell">
                        <button
                          class="icon-action drag-handle"
                          type="button"
                          [title]="messages.actions.dragTitle"
                          [attr.aria-label]="messages.actions.dragAria"
                          tabindex="-1"
                        >↕</button>
                      </td>
                      <td>{{ displaySortOrder(account) }}</td>
                      <td>{{ displayAccountName(account) }}</td>
                      <td>{{ accountType(account.Type) }}</td>
                      <td>{{ assetRoleLabel(account) }}</td>
                      <td>{{ dashboardVisibility(account) }}</td>
                      <td>{{ money(account.Balance) }}</td>
                      <td>{{ account.Currency }}</td>
                      <td class="actions-cell">
                        <button class="icon-action" type="button" [title]="messages.actions.editTitle" [attr.aria-label]="messages.actions.editAria" (click)="openEdit(account)">✎</button>
                        <button class="icon-action danger" type="button" [title]="messages.actions.deactivateTitle" [attr.aria-label]="messages.actions.deactivateAria" (click)="deactivate(account)">×</button>
                      </td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          </div>
        }

        @if (inactiveAccounts().length > 0) {
          <div class="table-section">
            <div class="table-section-header">
              <div>
                <h2>{{ messages.sections.inactive }}</h2>
                <p class="section-note">{{ messages.sections.inactiveNote }}</p>
              </div>
            </div>
            <div class="table-wrap">
              <table class="inactive-accounts-table">
                <thead>
                  <tr>
                    <th>{{ messages.columns.name }}</th>
                    <th>{{ messages.columns.type }}</th>
                    <th>{{ messages.columns.assetRole }}</th>
                    <th>{{ messages.columns.balance }}</th>
                    <th>{{ messages.columns.currency }}</th>
                    <th>{{ messages.columns.deactivatedAt }}</th>
                    <th class="inactive-actions-column"></th>
                  </tr>
                </thead>
                <tbody>
                  @for (account of inactiveAccounts(); track account.Code) {
                    <tr [class.liability-row]="isLiability(account)">
                      <td>{{ displayAccountName(account) }}</td>
                      <td>{{ accountType(account.Type) }}</td>
                      <td>{{ assetRoleLabel(account) }}</td>
                      <td>{{ money(account.Balance) }}</td>
                      <td>{{ account.Currency }}</td>
                      <td>{{ deactivatedLabel(account) }}</td>
                      <td class="actions-cell">
                        <button
                          class="icon-action danger"
                          type="button"
                          [title]="messages.actions.deleteTitle"
                          [attr.aria-label]="messages.actions.deleteAria"
                          (click)="deletePermanently(account)"
                        >
                          <svg aria-hidden="true" viewBox="0 0 24 24">
                            <path d="M9 3h6l1 2h4v2H4V5h4l1-2Zm-1 6h2v10H8V9Zm6 0h2v10h-2V9Zm-9 0h14l-1 12H6L5 9Z" />
                          </svg>
                        </button>
                      </td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          </div>
        }
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
            {{ messages.form.name }}
            <input formControlName="name" />
          </label>
          <label>
            {{ messages.form.type }}
            <select formControlName="type">
              <option value="ASSET">{{ messages.form.asset }}</option>
              <option value="LIABILITY">{{ messages.form.liability }}</option>
            </select>
          </label>
          <label>
            {{ messages.form.currency }}
            <input formControlName="currency" maxlength="3" />
          </label>
          <label>
            {{ messages.form.assetRole }}
            <select formControlName="asset_role">
              <option value="NORMAL">{{ messages.form.assetRoleNormal }}</option>
              <option value="BROKERAGE">{{ messages.form.assetRoleBrokerage }}</option>
              <option value="INVESTMENT">{{ messages.form.assetRoleInvestment }}</option>
            </select>
          </label>
          <p class="section-note checkbox-hint">{{ messages.form.assetRoleHint }}</p>
          <label class="checkbox-label">
            <input type="checkbox" formControlName="hide_from_dashboard" />
            {{ messages.form.hideFromDashboard }}
          </label>
          <p class="section-note checkbox-hint">{{ messages.form.hideFromDashboardHint }}</p>
          <button class="primary-button" type="submit" [disabled]="form.invalid || saving()">
            {{ saving() ? messages.actions.saving : messages.actions.save }}
          </button>
        </form>
      </aside>
    }
  `,
  styles: [`
    tbody tr:nth-child(even) td {
      background: var(--surface-soft);
    }

    .liability-row td {
      background: color-mix(in srgb, var(--danger-soft) 42%, var(--surface) 58%);
    }

    .liability-row:nth-child(even) td {
      background: color-mix(in srgb, var(--danger-soft) 50%, var(--surface-soft) 50%);
    }

    .drag-cell {
      width: 48px;
    }

    .drag-handle {
      cursor: grab;
      min-width: 40px;
      padding: 0;
    }

    .dragging-row td {
      opacity: 0.5;
    }

    .drag-target-row td {
      background: var(--accent-soft);
    }

    .checkbox-hint {
      margin-top: -8px;
    }

    .form-stack select:disabled,
    .form-stack input:disabled {
      opacity: 0.6;
      cursor: not-allowed;
      background: color-mix(in srgb, var(--surface-soft) 82%, var(--surface) 18%);
      color: var(--text-muted);
    }

    .inactive-accounts-table th,
    .inactive-accounts-table td {
      white-space: nowrap;
    }

    .inactive-accounts-table th:first-child,
    .inactive-accounts-table td:first-child {
      width: 100%;
      white-space: normal;
    }

    .inactive-accounts-table .inactive-actions-column,
    .inactive-accounts-table .actions-cell {
      width: 64px;
      text-align: center;
    }
  `],
})
export class AccountsComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly destroyRef = inject(DestroyRef);
  readonly messages = uiMessages.accounts;
  readonly commonMessages = uiMessages.common;
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly reordering = signal(false);
  readonly error = signal('');
  readonly accounts = signal<Account[]>([]);
  readonly panelOpen = signal(false);
  readonly editing = signal<Account | null>(null);
  readonly draggingCode = signal<string | null>(null);
  readonly dropTargetCode = signal<string | null>(null);
  readonly activeAccounts = computed(() => this.accounts().filter((account) => !account.DeactivatedAt));
  readonly inactiveAccounts = computed(() => this.accounts().filter((account) => Boolean(account.DeactivatedAt)));
  readonly form = this.fb.group({
    name: this.fb.nonNullable.control('', Validators.required),
    type: this.fb.nonNullable.control<AccountType>('ASSET', Validators.required),
    currency: this.fb.nonNullable.control('BRL', Validators.required),
    asset_role: this.fb.nonNullable.control<AccountAssetRole>('NORMAL'),
    hide_from_dashboard: this.fb.nonNullable.control(false),
  });

  constructor(
    private readonly accountsService: AccountsService,
    private readonly referenceData: ReferenceDataService,
    private readonly moneyVisibility: MoneyVisibilityService,
    private readonly toast: ToastService,
  ) {}

  ngOnInit(): void {
    this.watchAccountType();
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.accountsService.list().subscribe({
      next: (accounts) => {
        this.accounts.set(accounts);
        this.referenceData.accounts.set(accounts);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }

  openCreate(): void {
    this.editing.set(null);
    this.form.reset({ name: '', type: 'ASSET', currency: 'BRL', asset_role: 'NORMAL', hide_from_dashboard: false });
    this.panelOpen.set(true);
  }

  openEdit(account: Account): void {
    this.editing.set(account);
    this.form.reset({
      name: account.Name,
      type: account.Type,
      currency: account.Currency,
      asset_role: account.asset_role,
      hide_from_dashboard: account.hide_from_dashboard,
    });
    this.panelOpen.set(true);
  }

  closePanel(): void {
    this.panelOpen.set(false);
    this.editing.set(null);
  }

  save(): void {
    if (this.form.invalid) {
      return;
    }
    this.saving.set(true);
    const value = this.form.getRawValue();
    const editing = this.editing();
    const assetRole = value.type === 'ASSET' ? value.asset_role : 'NORMAL';
    const request = this.editing()
      ? this.accountsService.update(editing!.Code, {
          name: value.name,
          type: value.type,
          currency: value.currency.toUpperCase(),
          asset_role: assetRole,
          hide_from_dashboard: value.hide_from_dashboard,
        })
      : this.accountsService.create({
          name: value.name,
          type: value.type,
          currency: value.currency.toUpperCase(),
          asset_role: assetRole,
          hide_from_dashboard: value.hide_from_dashboard,
        });

    request.subscribe({
      next: () => {
        this.closePanel();
        this.reloadAccounts();
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.saving.set(false);
      },
      complete: () => this.saving.set(false),
    });
  }

  deactivate(account: Account): void {
    if (account.DeactivatedAt) {
      return;
    }

    const confirmed = window.confirm(deactivateAccountConfirmationMessage(account.Name));
    if (!confirmed) {
      return;
    }

    this.accountsService.deactivate(account.Code).subscribe({
      next: () => this.reloadAccounts(),
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  deletePermanently(account: Account): void {
    if (!account.DeactivatedAt) {
      return;
    }

    const confirmed = window.confirm(deleteAccountPermanentConfirmationMessage(account.Name));
    if (!confirmed) {
      return;
    }

    this.accountsService.deletePermanent(account.Code).subscribe({
      next: () => this.reloadAccounts(),
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  private reloadAccounts(): void {
    this.loading.set(true);
    this.referenceData.reload().subscribe({
      next: () => this.accounts.set(this.referenceData.accounts()),
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrency(value);
  }

  isLiability(account: Account): boolean {
    return account.Type === 'LIABILITY';
  }

  accountType(type: AccountType): string {
    return accountTypeLabel(type);
  }

  dashboardVisibility(account: Account): string {
    return account.hide_from_dashboard ? this.messages.dashboard.hidden : this.messages.dashboard.visible;
  }

  assetRoleLabel(account: Account): string {
    if (account.Type !== 'ASSET') {
      return this.messages.assetRole.normal;
    }
    switch (account.asset_role) {
      case 'BROKERAGE':
        return this.messages.assetRole.brokerage;
      case 'INVESTMENT':
        return this.messages.assetRole.investment;
      default:
        return this.messages.assetRole.normal;
    }
  }

  displayAccountName(account: Account): string {
    return this.referenceData.accountName(account.Code);
  }

  onDragStart(event: DragEvent, account: Account): void {
    if (this.reordering()) {
      event.preventDefault();
      return;
    }

    this.draggingCode.set(account.Code);
    this.dropTargetCode.set(account.Code);
    event.dataTransfer?.setData('text/plain', account.Code);
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
    }
  }

  onDragOver(event: DragEvent, account: Account): void {
    if (!this.draggingCode() || this.reordering()) {
      return;
    }

    event.preventDefault();
    this.dropTargetCode.set(account.Code);
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move';
    }
  }

  onDrop(event: DragEvent, targetAccount: Account): void {
    event.preventDefault();

    const sourceCode = this.draggingCode() ?? event.dataTransfer?.getData('text/plain') ?? null;
    this.onDragEnd();

    if (!sourceCode || sourceCode === targetAccount.Code || this.reordering()) {
      return;
    }

    const reordered = this.moveActiveAccount(sourceCode, targetAccount.Code);
    if (!reordered) {
      return;
    }

    this.reordering.set(true);
    this.accounts.set(reordered);

    this.accountsService.reorder(this.activeAccounts().map((account) => account.Code)).subscribe({
      next: () => this.reloadAccounts(),
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.reordering.set(false);
        this.reloadAccounts();
      },
      complete: () => this.reordering.set(false),
    });
  }

  onDragEnd(): void {
    this.draggingCode.set(null);
    this.dropTargetCode.set(null);
  }

  deactivatedLabel(account: Account): string {
    return account.DeactivatedAt ? new Intl.DateTimeFormat('pt-BR').format(new Date(account.DeactivatedAt)) : '-';
  }

  displaySortOrder(account: Account): string {
    return account.SortOrder?.toString() ?? '-';
  }

  private moveActiveAccount(sourceCode: string, targetCode: string): Account[] | null {
    const allAccounts = this.accounts();
    const activeAccounts = allAccounts.filter((account) => !account.DeactivatedAt);
    const inactiveAccounts = allAccounts.filter((account) => Boolean(account.DeactivatedAt));
    const sourceIndex = activeAccounts.findIndex((account) => account.Code === sourceCode);
    const targetIndex = activeAccounts.findIndex((account) => account.Code === targetCode);

    if (sourceIndex === -1 || targetIndex === -1) {
      return null;
    }

    const reorderedActive = [...activeAccounts];
    const [movedAccount] = reorderedActive.splice(sourceIndex, 1);
    reorderedActive.splice(targetIndex, 0, movedAccount);

    const normalizedActive = reorderedActive.map((account, index) => ({
      ...account,
      SortOrder: index + 1,
    }));

    return [...normalizedActive, ...inactiveAccounts];
  }

  private watchAccountType(): void {
    this.form.controls.type.valueChanges
      .pipe(
        startWith(this.form.controls.type.getRawValue()),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((type) => {
        if (type === 'ASSET') {
          this.form.controls.asset_role.enable({ emitEvent: false });
          return;
        }

        this.form.controls.asset_role.setValue('NORMAL', { emitEvent: false });
        this.form.controls.asset_role.disable({ emitEvent: false });
      });
  }
}
