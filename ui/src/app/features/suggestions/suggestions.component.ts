import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';

import { ReferenceDataService } from '../../data/reference-data.service';
import { SuggestionsService } from '../../data/suggestions.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { deleteSuggestionConfirmationMessage, uiMessages } from '../../shared/messages';
import {
  Category,
  CreateSuggestionPayload,
  Suggestion,
  SuggestionEntryType,
} from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

type SortColumn =
  | 'description_contains'
  | 'priority'
  | 'entry_type'
  | 'category_code'
  | 'account_code'
  | 'transfer_account_code';
type SortDirection = 'asc' | 'desc';

@Component({
  selector: 'app-suggestions',
  imports: [ReactiveFormsModule],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.page.eyebrow }}</p>
        <h1>{{ messages.page.title }}</h1>
      </div>
      <button class="primary-button" type="button" (click)="openCreate()">
        {{ messages.page.create }}
      </button>
    </section>
    <section class="panel">
      @if (loading()) {
        <p class="state-message">{{ messages.states.loading }}</p>
      } @else if (sortedSuggestions().length === 0) {
        <p class="state-message">{{ messages.states.empty }}</p>
      } @else {
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>
                  <button
                    class="sort-button"
                    type="button"
                    (click)="sortBy('description_contains')"
                  >
                    {{ messages.columns.descriptionContains }}
                    {{ sortIndicator('description_contains') }}
                  </button>
                </th>
                <th>
                  <button class="sort-button" type="button" (click)="sortBy('priority')">
                    {{ messages.columns.priority }} {{ sortIndicator('priority') }}
                  </button>
                </th>
                <th>
                  <button class="sort-button" type="button" (click)="sortBy('entry_type')">
                    {{ messages.columns.type }} {{ sortIndicator('entry_type') }}
                  </button>
                </th>
                <th>
                  <button class="sort-button" type="button" (click)="sortBy('category_code')">
                    {{ messages.columns.category }} {{ sortIndicator('category_code') }}
                  </button>
                </th>
                <th>
                  <button class="sort-button" type="button" (click)="sortBy('account_code')">
                    {{ messages.columns.account }} {{ sortIndicator('account_code') }}
                  </button>
                </th>
                <th>
                  <button
                    class="sort-button"
                    type="button"
                    (click)="sortBy('transfer_account_code')"
                  >
                    {{ messages.columns.transferAccount }}
                    {{ sortIndicator('transfer_account_code') }}
                  </button>
                </th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              @for (suggestion of sortedSuggestions(); track suggestion.id) {
                <tr>
                  <td>{{ suggestion.description_contains }}</td>
                  <td>{{ suggestion.priority }}</td>
                  <td>{{ entryTypeLabel(suggestion.entry_type) }}</td>
                  <td>{{ categoryName(suggestion.category_code) }}</td>
                  <td>{{ accountName(suggestion.account_code) }}</td>
                  <td>{{ accountName(suggestion.transfer_account_code) }}</td>
                  <td class="actions-cell">
                    <button
                      class="icon-action"
                      type="button"
                      [title]="messages.actions.editTitle"
                      [attr.aria-label]="messages.actions.editAria"
                      (click)="openEdit(suggestion)"
                    >
                      ✎
                    </button>
                    <button
                      class="icon-action danger"
                      type="button"
                      [title]="messages.actions.deleteTitle"
                      [attr.aria-label]="messages.actions.deleteAria"
                      (click)="remove(suggestion)"
                    >
                      ×
                    </button>
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
          <button class="ghost-button" type="button" (click)="closePanel()">
            {{ messages.actions.close }}
          </button>
        </div>
        <form class="form-stack" [formGroup]="form" (ngSubmit)="save()">
          <label>
            {{ messages.form.descriptionContains }}
            <input formControlName="description_contains" />
          </label>
          <label>
            {{ messages.form.priority }}
            <input type="number" min="1" formControlName="priority" />
          </label>
          <label>
            {{ messages.form.type }}
            <select formControlName="entry_type" (change)="onEntryTypeChange()">
              <option value="">{{ messages.form.noType }}</option>
              <option value="REVENUE">{{ messages.form.revenue }}</option>
              <option value="EXPENSE">{{ messages.form.expense }}</option>
              <option value="TRANSFER">{{ messages.form.transfer }}</option>
            </select>
          </label>
          <label>
            {{ messages.form.category }}
            <select formControlName="category_code">
              <option value="">{{ messages.form.noCategory }}</option>
              @for (category of categoryOptions(); track category.Code) {
                <option [value]="category.Code">{{ categoryOptionLabel(category) }}</option>
              }
            </select>
          </label>
          <label>
            {{ messages.form.account }}
            <select formControlName="account_code">
              <option value="">{{ messages.form.noAccount }}</option>
              @for (account of referenceData.accounts(); track account.Code) {
                <option [value]="account.Code">{{ account.Name }}</option>
              }
            </select>
          </label>
          @if (form.controls.entry_type.value === 'TRANSFER') {
            <label>
              {{ messages.form.transferAccount }}
              <select formControlName="transfer_account_code">
                <option value="">{{ messages.form.noTransferAccount }}</option>
                @for (account of referenceData.accounts(); track account.Code) {
                  <option [value]="account.Code">{{ account.Name }}</option>
                }
              </select>
            </label>
          }
          @if (formError()) {
            <p class="error-message">{{ formError() }}</p>
          }
          <button class="primary-button" type="submit" [disabled]="!canSave() || saving()">
            {{ saving() ? messages.actions.saving : messages.actions.save }}
          </button>
        </form>
      </aside>
    }
  `,
  styles: [
    `
      .sort-button {
        all: unset;
        cursor: pointer;
        font: inherit;
      }
    `,
  ],
})
export class SuggestionsComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  readonly messages = uiMessages.suggestions;
  readonly commonMessages = uiMessages.common;
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly error = signal('');
  readonly formError = signal('');
  readonly panelOpen = signal(false);
  readonly editing = signal<Suggestion | null>(null);
  readonly sortState = signal<{ column: SortColumn; direction: SortDirection }>({
    column: 'priority',
    direction: 'asc',
  });
  readonly leafCategories = computed(() => {
    const parentCodes = new Set(
      this.referenceData
        .activeCategories()
        .filter((category) => (category.SubCategories?.length ?? 0) > 0)
        .map((category) => category.Code),
    );
    return this.referenceData
      .activeFlatCategories()
      .filter((category) => !parentCodes.has(category.Code));
  });
  readonly sortedSuggestions = computed(() => {
    const { column, direction } = this.sortState();
    const multiplier = direction === 'asc' ? 1 : -1;
    return [...this.referenceData.suggestions()].sort((left, right) => {
      const leftValue = this.sortValue(left, column);
      const rightValue = this.sortValue(right, column);
      if (typeof leftValue === 'number' && typeof rightValue === 'number') {
        return (leftValue - rightValue) * multiplier;
      }
      return String(leftValue).localeCompare(String(rightValue), 'pt-BR') * multiplier;
    });
  });
  readonly form = this.fb.nonNullable.group({
    description_contains: ['', Validators.required],
    priority: [1, [Validators.required, Validators.min(1)]],
    entry_type: [''],
    category_code: [''],
    account_code: [''],
    transfer_account_code: [''],
  });

  categoryOptions(): Category[] {
    const entryType = this.form.controls.entry_type.value;
    const options = this.leafCategories();
    switch (entryType) {
      case 'REVENUE':
        return options.filter((category) => category.Type === 'INCOME');
      case 'EXPENSE':
        return options.filter((category) => category.Type === 'EXPENSE');
      case 'TRANSFER':
        return options.filter((category) => category.Type === 'MOVEMENT');
      default:
        return options;
    }
  }

  constructor(
    readonly referenceData: ReferenceDataService,
    private readonly suggestionsService: SuggestionsService,
    private readonly toast: ToastService,
  ) {}

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.referenceData.reload().subscribe({
      next: () => this.formError.set(''),
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }

  openCreate(): void {
    this.editing.set(null);
    this.formError.set('');
    this.form.reset({
      description_contains: '',
      priority: 1,
      entry_type: '',
      category_code: '',
      account_code: '',
      transfer_account_code: '',
    });
    this.panelOpen.set(true);
  }

  openEdit(suggestion: Suggestion): void {
    this.editing.set(suggestion);
    this.formError.set('');
    this.form.reset({
      description_contains: suggestion.description_contains,
      priority: suggestion.priority,
      entry_type: suggestion.entry_type ?? '',
      category_code: suggestion.category_code ?? '',
      account_code: suggestion.account_code ?? '',
      transfer_account_code: suggestion.transfer_account_code ?? '',
    });
    this.panelOpen.set(true);
  }

  closePanel(): void {
    this.panelOpen.set(false);
    this.editing.set(null);
    this.formError.set('');
  }

  save(): void {
    if (!this.canSave()) {
      this.formError.set(this.validationMessage());
      return;
    }

    this.saving.set(true);
    this.formError.set('');

    const payload = this.buildPayload();
    const request = this.editing()
      ? this.suggestionsService.update(this.editing()!.id, payload)
      : this.suggestionsService.create(payload);

    request.subscribe({
      next: (suggestion) => {
        this.referenceData.suggestions.update((items) =>
          this.editing()
            ? items.map((item) => (item.id === suggestion.id ? suggestion : item))
            : [...items, suggestion],
        );
        this.closePanel();
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.saving.set(false);
      },
      complete: () => this.saving.set(false),
    });
  }

  remove(suggestion: Suggestion): void {
    if (!window.confirm(deleteSuggestionConfirmationMessage(suggestion.description_contains))) {
      return;
    }

    this.suggestionsService.delete(suggestion.id).subscribe({
      next: () => {
        this.referenceData.suggestions.update((items) =>
          items.filter((item) => item.id !== suggestion.id),
        );
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  onEntryTypeChange(): void {
    const entryType = this.form.controls.entry_type.value;
    const categoryCode = this.form.controls.category_code.value;
    const categoryStillAllowed =
      !categoryCode || this.categoryOptions().some((category) => category.Code === categoryCode);
    if (!categoryStillAllowed) {
      this.form.patchValue({ category_code: '' });
    }
    if (entryType !== 'TRANSFER' && this.form.controls.transfer_account_code.value) {
      this.form.patchValue({ transfer_account_code: '' });
    }
  }

  canSave(): boolean {
    return this.form.valid && this.validationMessage() === '';
  }

  sortBy(column: SortColumn): void {
    this.sortState.update((current) => ({
      column,
      direction: current.column === column && current.direction === 'asc' ? 'desc' : 'asc',
    }));
  }

  sortIndicator(column: SortColumn): string {
    const current = this.sortState();
    if (current.column !== column) {
      return this.commonMessages.sortNeutral;
    }
    return current.direction === 'asc' ? this.commonMessages.sortAsc : this.commonMessages.sortDesc;
  }

  entryTypeLabel(entryType: SuggestionEntryType | null | undefined): string {
    switch (entryType) {
      case 'REVENUE':
        return this.messages.form.revenue;
      case 'EXPENSE':
        return this.messages.form.expense;
      case 'TRANSFER':
        return this.messages.form.transfer;
      default:
        return '-';
    }
  }

  categoryName(code: string | null | undefined): string {
    return code ? this.referenceData.categoryName(code) : '-';
  }

  accountName(code: string | null | undefined): string {
    return code ? this.referenceData.accountName(code) : '-';
  }

  categoryOptionLabel(category: Category): string {
    if (!category.ParentID) {
      return category.Name;
    }
    const parent = this.referenceData
      .flatCategories()
      .find((candidate) => candidate.ID === category.ParentID);
    return parent ? `${parent.Name} - ${category.Name}` : category.Name;
  }

  private validationMessage(): string {
    const { entry_type, category_code, account_code, transfer_account_code } =
      this.form.getRawValue();
    if (!entry_type && !category_code && !account_code && !transfer_account_code) {
      return this.messages.validation.targetRequired;
    }
    if (transfer_account_code && entry_type !== 'TRANSFER') {
      return this.messages.validation.transferTypeRequired;
    }
    return '';
  }

  private buildPayload(): CreateSuggestionPayload {
    const value = this.form.getRawValue();
    return {
      description_contains: value.description_contains,
      priority: value.priority,
      entry_type: value.entry_type as SuggestionEntryType | '',
      category_code: value.category_code,
      account_code: value.account_code,
      transfer_account_code: value.transfer_account_code,
    };
  }

  private sortValue(suggestion: Suggestion, column: SortColumn): string | number {
    switch (column) {
      case 'description_contains':
        return suggestion.description_contains;
      case 'priority':
        return suggestion.priority;
      case 'entry_type':
        return this.entryTypeLabel(suggestion.entry_type);
      case 'category_code':
        return this.categoryName(suggestion.category_code);
      case 'account_code':
        return this.accountName(suggestion.account_code);
      case 'transfer_account_code':
        return this.accountName(suggestion.transfer_account_code);
    }
  }
}
