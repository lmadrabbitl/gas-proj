import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';

import { ReferenceDataService } from '../../data/reference-data.service';
import { SuggestionsService } from '../../data/suggestions.service';
import { SuggestionsComponent } from './suggestions.component';

describe('SuggestionsComponent', () => {
  const suggestionsSignal = signal([
    {
      id: '1',
      description_contains: 'zebra',
      priority: 3,
      entry_type: 'EXPENSE' as const,
      category_code: 'quitanda',
      account_code: 'inter',
      transfer_account_code: null,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
    {
      id: '2',
      description_contains: 'alpha',
      priority: 1,
      entry_type: 'REVENUE' as const,
      category_code: 'salario',
      account_code: 'santander',
      transfer_account_code: null,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
  ]);

  const referenceData = {
    accounts: signal([
      { Code: 'inter', Name: 'Inter' },
      { Code: 'santander', Name: 'Santander' },
    ]),
    categories: signal([
      {
        ID: '10',
        Code: 'alimentacao',
        Name: 'Alimentação',
        Type: 'EXPENSE',
        SortOrder: 2,
        DeactivatedAt: null,
        SubCategories: [
          {
            ID: '11',
            ParentID: '10',
            Code: 'quitanda',
            Name: 'Quitanda',
            Type: 'EXPENSE',
            SortOrder: 2,
            DeactivatedAt: null,
          },
        ],
      },
      {
        ID: '20',
        Code: 'receita',
        Name: 'Receita',
        Type: 'INCOME',
        SortOrder: 1,
        DeactivatedAt: null,
        SubCategories: [
          {
            ID: '21',
            ParentID: '20',
            Code: 'salario',
            Name: 'Salário',
            Type: 'INCOME',
            SortOrder: 1,
            DeactivatedAt: null,
          },
        ],
      },
    ]),
    activeCategories: () =>
      referenceData
        .categories()
        .filter((category) => !category.DeactivatedAt)
        .map((category) => ({
          ...category,
          SubCategories: (category.SubCategories ?? []).filter((child) => !child.DeactivatedAt),
        })),
    flatCategories: () => [
      {
        ID: '20',
        Code: 'receita',
        Name: 'Receita',
        Type: 'INCOME',
        SortOrder: 1,
        DeactivatedAt: null,
        SubCategories: [
          {
            ID: '21',
            ParentID: '20',
            Code: 'salario',
            Name: 'Salário',
            Type: 'INCOME',
            SortOrder: 1,
            DeactivatedAt: null,
          },
        ],
      },
      {
        ID: '21',
        ParentID: '20',
        Code: 'salario',
        Name: 'Salário',
        Type: 'INCOME',
        SortOrder: 1,
        DeactivatedAt: null,
      },
      {
        ID: '10',
        Code: 'alimentacao',
        Name: 'Alimentação',
        Type: 'EXPENSE',
        SortOrder: 2,
        DeactivatedAt: null,
        SubCategories: [
          {
            ID: '11',
            ParentID: '10',
            Code: 'quitanda',
            Name: 'Quitanda',
            Type: 'EXPENSE',
            SortOrder: 2,
            DeactivatedAt: null,
          },
        ],
      },
      {
        ID: '11',
        ParentID: '10',
        Code: 'quitanda',
        Name: 'Quitanda',
        Type: 'EXPENSE',
        SortOrder: 2,
        DeactivatedAt: null,
      },
    ],
    activeFlatCategories: () => [
      ...referenceData.activeCategories(),
      ...referenceData.activeCategories().flatMap((category) => category.SubCategories ?? []),
    ],
    suggestions: suggestionsSignal,
    reload: vi.fn().mockReturnValue(of(void 0)),
    accountName: (code: string) =>
      referenceData.accounts().find((account) => account.Code === code)?.Name ?? code,
    categoryName: (code: string) =>
      referenceData.flatCategories().find((category) => category.Code === code)?.Name ?? code,
  };

  const suggestionsService = {
    create: vi.fn().mockImplementation((payload) =>
      of({
        id: '3',
        ...payload,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      }),
    ),
    update: vi.fn().mockImplementation((id, payload) =>
      of({
        id,
        ...suggestionsSignal().find((item) => item.id === id),
        ...payload,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      }),
    ),
    delete: vi.fn().mockReturnValue(of(void 0)),
  };

  beforeEach(async () => {
    referenceData.reload.mockClear();
    suggestionsService.create.mockClear();
    suggestionsService.update.mockClear();
    suggestionsService.delete.mockClear();
    suggestionsSignal.set([
      {
        id: '1',
        description_contains: 'zebra',
        priority: 3,
        entry_type: 'EXPENSE',
        category_code: 'quitanda',
        account_code: 'inter',
        transfer_account_code: null,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
      {
        id: '2',
        description_contains: 'alpha',
        priority: 1,
        entry_type: 'REVENUE',
        category_code: 'salario',
        account_code: 'santander',
        transfer_account_code: null,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]);

    await TestBed.configureTestingModule({
      imports: [SuggestionsComponent],
      providers: [
        { provide: ReferenceDataService, useValue: referenceData },
        { provide: SuggestionsService, useValue: suggestionsService },
      ],
    }).compileComponents();
  });

  it('defaults new suggestions to priority 1', () => {
    const fixture = TestBed.createComponent(SuggestionsComponent);
    const component = fixture.componentInstance;

    component.openCreate();

    expect(component.form.controls.priority.value).toBe(1);
  });

  it('sorts client-side by the selected column', () => {
    const fixture = TestBed.createComponent(SuggestionsComponent);
    fixture.detectChanges();
    const component = fixture.componentInstance;

    component.sortBy('description_contains');

    expect(component.sortedSuggestions().map((item) => item.description_contains)).toEqual([
      'alpha',
      'zebra',
    ]);
  });

  it('keeps category options in reference-data sort order', () => {
    const fixture = TestBed.createComponent(SuggestionsComponent);
    fixture.detectChanges();
    const component = fixture.componentInstance;

    expect(component.categoryOptions().map((category) => category.Code)).toEqual([
      'quitanda',
      'salario',
    ]);
  });

  it('creates a suggestion with the current form payload', () => {
    const fixture = TestBed.createComponent(SuggestionsComponent);
    const component = fixture.componentInstance;

    component.openCreate();
    component.form.patchValue({
      description_contains: 'padaria',
      priority: 1,
      entry_type: 'EXPENSE',
      category_code: 'quitanda',
      account_code: 'inter',
    });
    component.save();

    expect(suggestionsService.create).toHaveBeenCalledWith(
      expect.objectContaining({
        description_contains: 'padaria',
        priority: 1,
        entry_type: 'EXPENSE',
      }),
    );
  });

  it('shows the transfer account field only for transfer suggestions', () => {
    const fixture = TestBed.createComponent(SuggestionsComponent);
    const component = fixture.componentInstance;

    component.openCreate();
    fixture.detectChanges();

    expect(
      (fixture.nativeElement as HTMLElement).querySelector(
        'select[formcontrolname="transfer_account_code"]',
      ),
    ).toBeNull();

    component.form.patchValue({ entry_type: 'TRANSFER' });
    fixture.detectChanges();

    expect(
      (fixture.nativeElement as HTMLElement).querySelector(
        'select[formcontrolname="transfer_account_code"]',
      ),
    ).not.toBeNull();
  });

  it('allows saving a transfer suggestion without a transfer account', () => {
    const fixture = TestBed.createComponent(SuggestionsComponent);
    const component = fixture.componentInstance;

    component.openCreate();
    component.form.patchValue({
      description_contains: 'pix',
      priority: 1,
      entry_type: 'TRANSFER',
      category_code: '',
      account_code: '',
      transfer_account_code: '',
    });

    expect(component.canSave()).toBe(true);
  });
});
