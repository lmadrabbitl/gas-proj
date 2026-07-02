import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, Router } from '@angular/router';
import { BehaviorSubject, of, Subject } from 'rxjs';
import { vi } from 'vitest';

import { ReferenceDataService } from '../../data/reference-data.service';
import { TransactionsService } from '../../data/transactions.service';
import { UserConfigService } from '../../data/user-config.service';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { Account, Category, Transaction } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';
import { TransactionsComponent } from './transactions.component';

describe('TransactionsComponent', () => {
  const waitForDebounce = () => new Promise((resolve) => setTimeout(resolve, 300));
  const transactions = signal<Transaction[]>([]);

  const accounts = signal<Account[]>([
    {
      ID: '1',
      UserID: 'user-1',
      Code: 'santander',
      Name: 'Santander',
      Type: 'ASSET',
      Balance: 100000,
      Currency: 'BRL',
      hide_from_dashboard: false,
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
      DeactivatedAt: null,
    },
    {
      ID: '2',
      UserID: 'user-1',
      Code: 'xp',
      Name: 'XP',
      Type: 'LIABILITY',
      Balance: -5000,
      Currency: 'BRL',
      hide_from_dashboard: false,
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
      DeactivatedAt: null,
    },
    {
      ID: '3',
      UserID: 'user-1',
      Code: 'santander-antiga',
      Name: 'Santander',
      Type: 'ASSET',
      Balance: 0,
      Currency: 'BRL',
      hide_from_dashboard: false,
      CreatedAt: '2025-01-01T00:00:00Z',
      UpdatedAt: '2025-01-01T00:00:00Z',
      DeactivatedAt: '2026-06-01T12:00:00Z',
    },
  ]);

  const categories = signal<Category[]>([
    {
      ID: '1',
      UserID: 'user-1',
      ParentID: null,
      Code: 'despesas',
      Name: 'Despesas',
      Type: 'EXPENSE',
      Description: '',
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
      DeactivatedAt: null,
      SubCategories: [
        {
          ID: '2',
          UserID: 'user-1',
          ParentID: '1',
          Code: 'supermercado',
          Name: 'Supermercado',
          Type: 'EXPENSE',
          Description: '',
          CreatedAt: '2026-01-01T00:00:00Z',
          UpdatedAt: '2026-01-01T00:00:00Z',
          DeactivatedAt: null,
          SubCategories: [],
        },
        {
          ID: '3',
          UserID: 'user-1',
          ParentID: '1',
          Code: 'lazer',
          Name: 'Lazer',
          Type: 'EXPENSE',
          Description: '',
          CreatedAt: '2026-01-01T00:00:00Z',
          UpdatedAt: '2026-01-01T00:00:00Z',
          DeactivatedAt: null,
          SubCategories: [],
        },
      ],
    },
    {
      ID: '4',
      UserID: 'user-1',
      ParentID: null,
      Code: 'salario',
      Name: 'Salário',
      Type: 'INCOME',
      Description: '',
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
      DeactivatedAt: null,
      SubCategories: [],
    },
    {
      ID: '5',
      UserID: 'user-1',
      ParentID: null,
      Code: 'movimentacao',
      Name: 'Movimentação',
      Type: 'MOVEMENT',
      Description: '',
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
      DeactivatedAt: null,
      SubCategories: [
        {
          ID: '6',
          UserID: 'user-1',
          ParentID: '5',
          Code: 'transferencias',
          Name: 'Transferências',
          Type: 'MOVEMENT',
          Description: '',
          CreatedAt: '2026-01-01T00:00:00Z',
          UpdatedAt: '2026-01-01T00:00:00Z',
          DeactivatedAt: null,
          SubCategories: [],
        },
      ],
    },
  ]);

  const flatCategories = () => [
    ...categories(),
    ...categories().flatMap((category) => category.SubCategories ?? []),
  ];
  const activeCategories = () => categories().filter((category) => !category.DeactivatedAt);
  const activeFlatCategories = () => [
    ...activeCategories(),
    ...activeCategories().flatMap((category) => category.SubCategories ?? []),
  ];

  const transactionListConfig = signal({
    page_size: 50,
    show_total: false,
  });

  const transactionsService = {
    list: vi.fn().mockImplementation(() => of({
      transactions: transactions(),
      pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 },
      config: transactionListConfig(),
    })),
    create: vi.fn().mockReturnValue(of({})),
    update: vi.fn().mockReturnValue(of({})),
    updateMany: vi.fn().mockReturnValue(of(2)),
    delete: vi.fn(),
  };

  const referenceData = {
    load: vi.fn().mockReturnValue(of(void 0)),
    reload: vi.fn().mockReturnValue(of(void 0)),
    accounts,
    categories,
    activeCategories,
    flatCategories,
    activeFlatCategories,
    accountName: (code: string) => {
      const account = accounts().find((candidate) => candidate.Code === code);
      if (!account) {
        return code;
      }
      if (!account.DeactivatedAt) {
        return account.Name;
      }

      const normalized = account.Name.trim().toLocaleLowerCase('pt-BR');
      const hasCollision = accounts().some(
        (candidate) =>
          candidate.Code !== account.Code &&
          candidate.Name.trim().toLocaleLowerCase('pt-BR') === normalized,
      );
      if (!hasCollision) {
        return account.Name;
      }

      return `${account.Name} (desativada em 01/06/2026)`;
    },
    categoryName: (code: string) => flatCategories().find((category) => category.Code === code)?.Name ?? code,
  };

  const moneyVisibility = {
    formatCurrencyAbsolute: (value: number) => `R$ ${Math.abs(value)}`,
  };

  const toast = {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    clear: vi.fn(),
  };

  const userConfig = {
    transactionListConfig,
    updateTransactionListConfig: vi.fn().mockImplementation((config: { page_size: number; show_total: boolean }) => {
      transactionListConfig.set(config);
      return of({
        language: 'pt-BR',
        settings: {
          transactions: {
            list: config,
          },
          reports: {
            show_empty_categories: true,
          },
          investments: {
            portfolios: {
              rebalance_tolerance_basis_point: 50,
              suggestion_strategy: 'BEST_NEXT_SHARE',
            },
            integration: {
              watched_category_ids: [],
            },
          },
          ui: {
            hide_amounts: false,
          },
        },
      });
    }),
    syncTransactionListConfig: vi.fn().mockImplementation((config: { page_size: number; show_total: boolean }) => {
      transactionListConfig.set(config);
    }),
  };

  const queryParamMap$ = new BehaviorSubject(convertToParamMap({}));
  const activatedRoute = {
    snapshot: {
      queryParamMap: convertToParamMap({}),
    },
    queryParamMap: queryParamMap$.asObservable(),
  };
  const router = {
    navigate: vi.fn().mockImplementation((_commands: unknown[], options?: { queryParams?: Record<string, string | null> }) => {
      const nextQueryParams = Object.fromEntries(
        Object.entries(options?.queryParams ?? {}).filter(([, value]) => value !== null && value !== ''),
      );
      const paramMap = convertToParamMap(nextQueryParams);
      activatedRoute.snapshot.queryParamMap = paramMap;
      queryParamMap$.next(paramMap);
      return Promise.resolve(true);
    }),
  };

  beforeEach(async () => {
    transactions.set([]);
    transactionListConfig.set({ page_size: 50, show_total: false });
    transactionsService.list.mockClear();
    transactionsService.create.mockClear();
    transactionsService.update.mockClear();
    transactionsService.updateMany.mockClear();
    transactionsService.delete.mockClear();
    referenceData.load.mockClear();
    referenceData.reload.mockClear();
    userConfig.updateTransactionListConfig.mockClear();
    userConfig.syncTransactionListConfig.mockClear();
    toast.success.mockClear();
    toast.error.mockClear();
    toast.info.mockClear();
    toast.clear.mockClear();
    router.navigate.mockClear();
    const emptyParamMap = convertToParamMap({});
    activatedRoute.snapshot.queryParamMap = emptyParamMap;
    queryParamMap$.next(emptyParamMap);

    await TestBed.configureTestingModule({
      imports: [TransactionsComponent],
      providers: [
        { provide: TransactionsService, useValue: transactionsService },
        { provide: ReferenceDataService, useValue: referenceData },
        { provide: UserConfigService, useValue: userConfig },
        { provide: MoneyVisibilityService, useValue: moneyVisibility },
        { provide: ToastService, useValue: toast },
        { provide: ActivatedRoute, useValue: activatedRoute },
        { provide: Router, useValue: router },
      ],
    }).compileComponents();
  });

  it('reloads transactions when filters change', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();

    expect(transactionsService.list).toHaveBeenCalledTimes(1);

    fixture.componentInstance.filters.patchValue({
      account_codes: ['santander', 'xp'],
      category_codes: ['supermercado'],
      operations: ['credit', 'transfer'],
    });
    await waitForDebounce();

    expect(transactionsService.list).toHaveBeenCalledTimes(2);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({
      account_code: ['santander', 'xp'],
      category_code: ['supermercado'],
      operation: ['credit', 'transfer'],
      limit: 50,
      page: 1,
    }));
    expect(router.navigate).toHaveBeenCalledWith([], expect.objectContaining({
      queryParams: expect.objectContaining({
        account_code: 'santander,xp',
        category_code: 'supermercado',
        operation: 'credit,transfer',
      }),
    }));
  });

  it('hydrates filters from query params before the first transactions load', async () => {
    const initialParams = convertToParamMap({
      category_code: 'supermercado',
      from_date: '2026-01-01',
      to_date: '2026-01-31',
    });
    activatedRoute.snapshot.queryParamMap = initialParams;
    queryParamMap$.next(initialParams);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();

    expect(fixture.componentInstance.filters.getRawValue()).toEqual(expect.objectContaining({
      category_codes: ['supermercado'],
      from_date: '01/01/2026',
      to_date: '31/01/2026',
    }));
    expect(transactionsService.list).toHaveBeenCalledTimes(1);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({
      category_code: ['supermercado'],
      from_date: '2026-01-01',
      to_date: '2026-01-31',
    }));
  });

  it('shows a settings button instead of a create button and auto-filters description terms', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const settingsButton = host.querySelector('button[aria-label="Abrir configurações da lista de transações"]');

    expect(host.querySelector('button[type="submit"]')).toBeNull();
    expect(host.textContent).not.toContain('Nova transação');
    expect(settingsButton).not.toBeNull();
    expect(settingsButton?.textContent?.trim()).toBe('');

    fixture.componentInstance.filters.patchValue({ description: 'de -juros dividendo ir' });
    await waitForDebounce();

    expect(transactionsService.list).toHaveBeenCalledTimes(2);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({
      description: 'de -juros dividendo ir',
      limit: 50,
      page: 1,
    }));
  });

  it('persists page settings through user config and reloads with the chosen page size', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(transactionsService.list).toHaveBeenCalledTimes(1);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({ limit: 50 }));

    fixture.componentInstance.openSettings();
    fixture.detectChanges();

    fixture.componentInstance.settingsForm.patchValue({
      page_size: 125,
      show_total: true,
    });
    fixture.componentInstance.saveSettings();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(transactionsService.list).toHaveBeenCalledTimes(2);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({ limit: 125, page: 1 }));
    expect(userConfig.updateTransactionListConfig).toHaveBeenCalledWith({ page_size: 125, show_total: true });
  });

  it('accepts page size values coming from the form as strings', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(transactionsService.list).toHaveBeenCalledTimes(1);

    fixture.componentInstance.openSettings();
    fixture.detectChanges();

    fixture.componentInstance.settingsForm.patchValue({
      page_size: '1000' as never,
      show_total: false,
    });
    fixture.componentInstance.saveSettings();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(transactionsService.list).toHaveBeenCalledTimes(2);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({ limit: 1000, page: 1 }));
    expect(userConfig.updateTransactionListConfig).toHaveBeenCalledWith({ page_size: 1000, show_total: false });
  });

  it('renders a total row when enabled in settings', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'salario',
        description: 'Salario',
        date: '2026-01-10T00:00:00Z',
        account_code: 'santander',
        amount: 5000,
      },
      {
        id: 'tx-2',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
    ]);
    transactionListConfig.set({ page_size: 50, show_total: true });

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const totalRow = host.querySelector('tfoot .total-row');

    expect(totalRow?.textContent).toContain('Total da página');
    expect(totalRow?.textContent).toContain('R$ 3800');
  });

  it('shows the selected transactions total in the footer when rows are selected', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'salario',
        description: 'Salario',
        date: '2026-01-10T00:00:00Z',
        account_code: 'santander',
        amount: 5000,
      },
      {
        id: 'tx-2',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
      {
        id: 'tx-3',
        category_code: 'lazer',
        description: 'Cinema',
        date: '2026-01-12T00:00:00Z',
        account_code: 'nubank',
        amount: -300,
      },
    ]);
    transactionListConfig.set({ page_size: 50, show_total: true });

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();

    fixture.componentInstance.selectedTransactionIds.set(['tx-2', 'tx-3']);
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const totalRow = host.querySelector('tfoot .total-row');

    expect(totalRow?.textContent).toContain('Total dos itens selecionados');
    expect(totalRow?.textContent).toContain('R$ 1500');
  });

  it('waits for a complete date before auto-filtering', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();

    expect(transactionsService.list).toHaveBeenCalledTimes(1);

    fixture.componentInstance.filters.patchValue({ from_date: '01/0' });
    await waitForDebounce();
    expect(transactionsService.list).toHaveBeenCalledTimes(1);

    fixture.componentInstance.filters.patchValue({ from_date: '01/05/2026' });
    await waitForDebounce();
    expect(transactionsService.list).toHaveBeenCalledTimes(2);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({
      from_date: '2026-05-01',
      limit: 50,
    }));
  });

  it('formats date filters with automatic slashes while typing', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const fromInput = host.querySelector('input[formcontrolname="from_date"]') as HTMLInputElement;

    fromInput.value = '01052026';
    fromInput.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
    await waitForDebounce();

    expect(fixture.componentInstance.filters.controls.from_date.value).toBe('01/05/2026');
    expect(fromInput.value).toBe('01/05/2026');
    expect(transactionsService.list).toHaveBeenCalledTimes(2);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({
      from_date: '2026-05-01',
      limit: 50,
    }));
  });

  it('filters from checkbox picks and closes the menu on outside click', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const trigger = host.querySelector('[data-multi-select="account"] .multi-select-trigger') as HTMLButtonElement;

    trigger.click();
    fixture.detectChanges();
    expect(host.querySelector('[data-multi-select="account"] .multi-select-menu')).not.toBeNull();

    const checkbox = host.querySelector('[data-multi-select="account"] input[type="checkbox"]') as HTMLInputElement;
    checkbox.click();
    fixture.detectChanges();
    await waitForDebounce();

    expect(transactionsService.list).toHaveBeenCalledTimes(2);
    expect(fixture.componentInstance.filters.controls.account_codes.value).toEqual(['santander']);

    document.body.click();
    fixture.detectChanges();
    expect(host.querySelector('[data-multi-select="account"] .multi-select-menu')).toBeNull();
  });

  it('reloads reference data after deleting a transaction', async () => {
    transactionsService.delete.mockReturnValue(of(void 0));
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();

    fixture.componentInstance.delete({
      id: 'tx-1',
      category_code: 'supermercado',
      description: 'Mercado',
      date: '2026-01-11T00:00:00Z',
      account_code: 'santander',
      amount: -1200,
    });

    expect(transactionsService.delete).toHaveBeenCalledWith('tx-1');
    expect(referenceData.reload).toHaveBeenCalledTimes(1);
    confirmSpy.mockRestore();
  });

  it('shows parent category headers without making them selectable', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const trigger = host.querySelector('[data-multi-select="category"] .multi-select-trigger') as HTMLButtonElement;

    trigger.click();
    fixture.detectChanges();

    const labels = [...host.querySelectorAll('[data-multi-select="category"] .multi-select-group-label')].map((node) => node.textContent?.trim());
    expect(labels).toContain('Despesas');

    const headerRow = [...host.querySelectorAll('[data-multi-select="category"] .multi-select-group > div')].find((node) => node.textContent?.includes('Despesas'));
    expect(headerRow?.querySelector('input')).toBeNull();

    const categoryCheckboxLabels = [...host.querySelectorAll('[data-multi-select="category"] .multi-select-option span')].map((node) => node.textContent?.trim());
    expect(categoryCheckboxLabels).toContain('Supermercado');
    expect(categoryCheckboxLabels).toContain('Lazer');
    expect(categoryCheckboxLabels).toContain('Salário');
  });

  it('groups deactivated accounts under a dedicated filter section', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const trigger = host.querySelector('[data-multi-select="account"] .multi-select-trigger') as HTMLButtonElement;

    trigger.click();
    fixture.detectChanges();

    const labels = [...host.querySelectorAll('[data-multi-select="account"] .multi-select-group-label')].map((node) => node.textContent?.trim());
    expect(labels).toContain('Desativadas');

    const optionLabels = [...host.querySelectorAll('[data-multi-select="account"] .multi-select-option span')].map((node) => node.textContent?.trim());
    expect(optionLabels).toContain('Santander');
    expect(optionLabels).toContain('XP');
    expect(optionLabels).toContain('Santander (desativada em 01/06/2026)');
  });

  it('shows operation filter options and reloads when selected', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const trigger = host.querySelector('[data-multi-select="operation"] .multi-select-trigger') as HTMLButtonElement;

    trigger.click();
    fixture.detectChanges();

    const optionLabels = [...host.querySelectorAll('[data-multi-select="operation"] .multi-select-option span')].map((node) => node.textContent?.trim());
    expect(optionLabels).toEqual(expect.arrayContaining(['Receita', 'Despesa', 'Transferência']));

    const checkbox = host.querySelector('[data-multi-select="operation"] input[type="checkbox"]') as HTMLInputElement;
    checkbox.click();
    fixture.detectChanges();
    await waitForDebounce();

    expect(transactionsService.list).toHaveBeenCalledTimes(2);
    expect(fixture.componentInstance.filters.controls.operations.value).toEqual(['credit']);
    expect(transactionsService.list).toHaveBeenLastCalledWith(expect.objectContaining({
      operation: ['credit'],
      limit: 50,
      page: 1,
    }));
  });

  it('shows bulk edit actions when at least one transaction is selected', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const checkbox = host.querySelector('tbody input[type="checkbox"]') as HTMLInputElement;
    checkbox.click();
    fixture.detectChanges();

    expect(host.querySelector('button[aria-label="Editar transações selecionadas"]')).not.toBeNull();
  });

  it('toggles selection when clicking anywhere in the transaction row', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const row = host.querySelector('tbody tr') as HTMLTableRowElement;

    row.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.selectedTransactionIds()).toEqual(['tx-1']);

    row.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.selectedTransactionIds()).toEqual([]);
  });

  it('selects and clears only the current page transactions with the select all button', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
      {
        id: 'tx-2',
        category_code: 'lazer',
        description: 'Cinema',
        date: '2026-01-12T00:00:00Z',
        account_code: 'santander',
        amount: -800,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    expect(host.textContent).not.toContain('Selecionar todas');

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.detectChanges();

    const selectAllButton = host.querySelector('button[aria-label="Selecionar todas"]') as HTMLButtonElement;

    selectAllButton.click();
    fixture.detectChanges();

    expect(fixture.componentInstance.selectedTransactionIds()).toEqual(['tx-1', 'tx-2']);
    expect(host.textContent).toContain('2 selecionadas');

    const clearButton = host.querySelector('button[aria-label="Limpar seleção"]') as HTMLButtonElement;

    clearButton.click();
    fixture.detectChanges();

    expect(fixture.componentInstance.selectedTransactionIds()).toEqual([]);
  });

  it('opens multi edit with transfer already selected for transfer-only rows', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Transfer 1',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
        transfer_id: 1,
        account_transfer: 'xp',
      },
      {
        id: 'tx-2',
        category_code: 'supermercado',
        description: 'Transfer 2',
        date: '2026-01-12T00:00:00Z',
        account_code: 'santander',
        amount: -800,
        transfer_id: 2,
        account_transfer: 'xp',
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const checkboxes = host.querySelectorAll('tbody input[type="checkbox"]');
    (checkboxes[0] as HTMLInputElement).click();
    (checkboxes[1] as HTMLInputElement).click();
    fixture.detectChanges();

    const editButton = host.querySelector('button[aria-label="Editar transações selecionadas"]') as HTMLButtonElement;
    editButton.click();
    fixture.detectChanges();

    expect(fixture.componentInstance.form.controls.is_transfer.value).toBe(true);
    expect(host.querySelector('select[formcontrolname="account_transfer"]')).not.toBeNull();
  });

  it('blocks bulk edit for mixed transfer and non-transfer selections', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
      {
        id: 'tx-2',
        category_code: 'supermercado',
        description: 'Transfer',
        date: '2026-01-12T00:00:00Z',
        account_code: 'santander',
        amount: -800,
        transfer_id: 1,
        account_transfer: 'xp',
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const checkboxes = host.querySelectorAll('tbody input[type="checkbox"]');
    (checkboxes[0] as HTMLInputElement).click();
    (checkboxes[1] as HTMLInputElement).click();
    fixture.detectChanges();

    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    expect(toast.error).toHaveBeenCalledWith('Selecione apenas transações do mesmo tipo para editar em lote.');
    expect(host.querySelector('.side-panel')).toBeNull();
  });

  it('disables save when no single-edit field changed and sends only changed fields', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const checkbox = host.querySelector('tbody input[type="checkbox"]') as HTMLInputElement;
    checkbox.click();
    fixture.detectChanges();

    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    const saveButton = host.querySelector('.side-panel button[type="submit"]') as HTMLButtonElement;
    expect(saveButton.disabled).toBe(true);

    fixture.componentInstance.form.patchValue({ category_code: 'lazer' });
    fixture.detectChanges();

    expect(saveButton.disabled).toBe(false);

    fixture.componentInstance.save();

    expect(transactionsService.update).toHaveBeenCalledWith('tx-1', { category_code: 'lazer' });
  });

  it('shows single-edit dates in dd/mm/yyyy format', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    expect(fixture.componentInstance.form.controls.date.value).toBe('11/01/2026');
  });

  it('groups categories by parent in the edit form selector', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const categorySelect = host.querySelector('.side-panel select[formcontrolname="category_code"]') as HTMLSelectElement;
    const groupLabels = Array.from(categorySelect.querySelectorAll('optgroup')).map((group) => group.label);
    const standaloneOptions = Array.from(categorySelect.querySelectorAll(':scope > option')).map((option) => option.textContent?.trim());

    expect(groupLabels).toContain('Despesas');
    expect(groupLabels).not.toContain('Movimentação');
    expect(categorySelect.textContent).toContain('Supermercado');
    expect(categorySelect.textContent).toContain('Lazer');
    expect(categorySelect.textContent).not.toContain('Transferências');
    expect(standaloneOptions).toContain('Salário');
  });

  it('shows only movement categories when editing a transfer', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'transferencias',
        description: 'Transferência',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
        transfer_id: 1,
        account_transfer: 'xp',
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const categorySelect = host.querySelector('.side-panel select[formcontrolname="category_code"]') as HTMLSelectElement;

    expect(categorySelect.textContent).toContain('Transferências');
    expect(categorySelect.textContent).not.toContain('Supermercado');
    expect(categorySelect.textContent).not.toContain('Salário');
  });

  it('hides the dashboard exclusion option when editing a transfer', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'transferencias',
        description: 'Transferência',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
        transfer_id: 1,
        account_transfer: 'xp',
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    expect(host.textContent).not.toContain('Excluir do dashboard');
  });

  it('clears a movement category when transfer is unchecked in the edit form', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'transferencias',
        description: 'Transferência',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
        transfer_id: 1,
        account_transfer: 'xp',
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    fixture.componentInstance.form.patchValue({ is_transfer: false });
    fixture.detectChanges();

    expect(fixture.componentInstance.form.controls.category_code.value).toBe('');
  });

  it('sends the dashboard exclusion flag when creating a non-transfer transaction', async () => {
    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();

    fixture.componentInstance.form.patchValue({
      date: '11/01/2026',
      description: 'Bonus',
      amount: '123,45',
      account_code: 'santander',
      category_code: 'salario',
      is_transfer: false,
      exclude_from_dashboard: true,
    });

    fixture.componentInstance.save();

    expect(transactionsService.create).toHaveBeenCalledWith({
      date: '2026-01-11T00:00:00.000Z',
      description: 'Bonus',
      amount: 12345,
      account_code: 'santander',
      category_code: 'salario',
      is_transfer: false,
      account_transfer: null,
      exclude_from_dashboard: true,
    });
  });

  it('sends exclude_from_dashboard only when changed in single edit', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
        exclude_from_dashboard: false,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    fixture.componentInstance.form.patchValue({ exclude_from_dashboard: true });
    fixture.componentInstance.form.controls.exclude_from_dashboard.markAsDirty();
    fixture.componentInstance.save();

    expect(transactionsService.update).toHaveBeenCalledWith('tx-1', {
      exclude_from_dashboard: true,
    });
  });

  it('sends bulk updates with selected ids and changed fields only', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
      },
      {
        id: 'tx-2',
        category_code: 'lazer',
        description: 'Cinema',
        date: '2026-01-12T00:00:00Z',
        account_code: 'santander',
        amount: -800,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.componentInstance.toggleTransactionSelection('tx-2');
    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    fixture.componentInstance.form.patchValue({ category_code: 'supermercado' });
    fixture.detectChanges();

    fixture.componentInstance.save();

    expect(transactionsService.updateMany).toHaveBeenCalledWith({
      ids: ['tx-1', 'tx-2'],
      category_code: 'supermercado',
    });
  });

  it('sends exclude_from_dashboard in bulk edit only after the checkbox changes', async () => {
    transactions.set([
      {
        id: 'tx-1',
        category_code: 'supermercado',
        description: 'Mercado',
        date: '2026-01-11T00:00:00Z',
        account_code: 'santander',
        amount: -1200,
        exclude_from_dashboard: false,
      },
      {
        id: 'tx-2',
        category_code: 'lazer',
        description: 'Cinema',
        date: '2026-01-12T00:00:00Z',
        account_code: 'santander',
        amount: -800,
        exclude_from_dashboard: false,
      },
    ]);

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    fixture.componentInstance.toggleTransactionSelection('tx-1');
    fixture.componentInstance.toggleTransactionSelection('tx-2');
    fixture.componentInstance.openSelectedEdit();
    fixture.detectChanges();

    fixture.componentInstance.form.patchValue({ exclude_from_dashboard: true });
    fixture.componentInstance.form.controls.exclude_from_dashboard.markAsDirty();
    fixture.componentInstance.save();

    expect(transactionsService.updateMany).toHaveBeenCalledWith({
      ids: ['tx-1', 'tx-2'],
      exclude_from_dashboard: true,
    });
  });

  it('shows skeleton rows while the transaction list is loading', async () => {
    const response$ = new Subject<{
      transactions: Transaction[];
      pagination: { page: number; page_size: number; total: number; total_pages: number };
      config: { page_size: number; show_total: boolean };
    }>();
    transactionsService.list.mockReturnValueOnce(response$.asObservable());

    const fixture = TestBed.createComponent(TransactionsComponent);
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    expect(host.querySelector('[data-testid="transactions-skeleton"]')).not.toBeNull();

    response$.next({
      transactions: [
        {
          id: 'tx-1',
          category_code: 'supermercado',
          description: 'Mercado',
          date: '2026-01-11T00:00:00Z',
          account_code: 'santander',
          amount: -1200,
        },
      ],
      pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 },
      config: { page_size: 50, show_total: false },
    });
    response$.complete();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(host.querySelector('[data-testid="transactions-skeleton"]')).toBeNull();
    expect(host.textContent).toContain('Mercado');
  });
});
