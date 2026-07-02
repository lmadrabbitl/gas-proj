import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';

import { ReferenceDataService } from '../../data/reference-data.service';
import { TransactionsService } from '../../data/transactions.service';
import { InsertTransactionsComponent } from './insert-transactions.component';

describe('InsertTransactionsComponent', () => {
  const categoryTree = [
    {
      Code: 'alimentacao',
      Name: 'Alimentacao',
      Type: 'EXPENSE',
      SubCategories: [
        { Code: 'cartao', Name: 'Cartao', Type: 'EXPENSE' },
        { Code: 'salario', Name: 'Salario', Type: 'REVENUE' },
      ],
    },
    {
      Code: 'movimentacoes',
      Name: 'Movimentacoes',
      Type: 'MOVEMENT',
      SubCategories: [
        { Code: 'transferencias', Name: 'Transferencias', Type: 'MOVEMENT' },
      ],
    },
  ];

  const flatCategories = [
    {
      Code: 'alimentacao',
      Name: 'Alimentacao',
      Type: 'EXPENSE',
      SubCategories: [
        { Code: 'cartao', Name: 'Cartão', Type: 'EXPENSE' },
        { Code: 'salario', Name: 'Salário', Type: 'REVENUE' },
      ],
    },
    { Code: 'cartao', Name: 'Cartão', Type: 'EXPENSE' },
    { Code: 'salario', Name: 'Salário', Type: 'REVENUE' },
    {
      Code: 'movimentacoes',
      Name: 'Movimentacoes',
      Type: 'MOVEMENT',
      SubCategories: [
        { Code: 'transferencias', Name: 'Transferencias', Type: 'MOVEMENT' },
      ],
    },
    { Code: 'transferencias', Name: 'Transferencias', Type: 'MOVEMENT' },
  ];

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-06-15T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [InsertTransactionsComponent],
      providers: [
        {
          provide: ReferenceDataService,
          useValue: {
            load: () => of(void 0),
            reload: vi.fn(() => of(void 0)),
            accounts: () => [
              { Code: 'santander', Name: 'Santander', Balance: 100000 },
              { Code: 'xp', Name: 'XP', Balance: 50000 },
            ],
            categories: () => categoryTree,
            activeCategories: () => categoryTree,
            flatCategories: () => flatCategories,
            activeFlatCategories: () => flatCategories,
            suggestions: () => [
              {
                id: '1',
                description_contains: 'mercado',
                priority: 2,
                entry_type: 'EXPENSE',
                category_code: 'cartao',
                account_code: 'xp',
                transfer_account_code: null,
                created_at: '2026-01-01T00:00:00Z',
                updated_at: '2026-01-01T00:00:00Z',
              },
              {
                id: '2',
                description_contains: 'mercado extra',
                priority: 2,
                entry_type: 'EXPENSE',
                category_code: 'cartao',
                account_code: 'santander',
                transfer_account_code: null,
                created_at: '2026-01-01T00:00:00Z',
                updated_at: '2026-01-01T00:00:00Z',
              },
              {
                id: '3',
                description_contains: 'pix',
                priority: 1,
                entry_type: 'TRANSFER',
                category_code: 'transferencias',
                account_code: 'santander',
                transfer_account_code: 'xp',
                created_at: '2026-01-01T00:00:00Z',
                updated_at: '2026-01-01T00:00:00Z',
              },
            ],
          },
        },
        {
          provide: TransactionsService,
          useValue: {
            createMany: vi.fn(() => of([])),
          },
        },
      ],
    }).compileComponents();
  });

  it('starts with 10 draft rows', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    vi.runAllTimers();

    expect(fixture.componentInstance.rows()).toHaveLength(10);
  });

  it('does not render the dashboard visibility column in the insert grid', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.textContent).not.toContain('Painel');
    expect(compiled.querySelectorAll('input[type="checkbox"]')).toHaveLength(0);
  });

  it('focuses the first description field on load', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const firstDescription = fixture.nativeElement.querySelector(
      'tbody tr:first-child td:first-child input.grid-input',
    ) as HTMLInputElement | null;
    const focusSpy = firstDescription ? vi.spyOn(firstDescription, 'focus') : null;

    vi.runAllTimers();

    expect(focusSpy).not.toBeNull();
    expect(focusSpy).toHaveBeenCalled();
  });

  it('fills today date when description starts', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.description = 'Mercado';
    fixture.componentInstance.onDescriptionChange(row);

    expect(row.date).toMatch(/^\d{2}\/\d{2}\/\d{4}$/);
    expect(row.date).toBe('15/06/2026');
  });

  it('filters category options by entry type', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.type = 'EXPENSE';
    expect(fixture.componentInstance.categoryOptions(row).map((category) => category.Code)).toEqual(
      ['cartao', 'salario'],
    );

    row.type = 'TRANSFER';
    expect(fixture.componentInstance.categoryOptions(row).map((category) => category.Code)).toEqual(
      ['transferencias'],
    );
  });

  it('formats draft dates with automatic slashes while typing', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    fixture.componentInstance.onDateInput(row, '01052026');
    expect(row.date).toBe('01/05/2026');

    fixture.componentInstance.onDateInput(row, '0105');
    expect(row.date).toBe('01/05');

    fixture.componentInstance.onDateInput(row, '18');
    expect(row.date).toBe('18');
  });

  it('matches categories without requiring accents', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.description = 'Cashback';
    row.amount = '45,00';
    row.type = 'EXPENSE';
    row.date = '14';
    row.category = 'Cartao';
    row.accountCode = 'santander';

    expect(fixture.componentInstance.rowValidation(row).valid).toBe(true);
  });

  it('accepts a unique partial category name', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.description = 'Pagamento';
    row.amount = '100,00';
    row.type = 'REVENUE';
    row.date = '14';
    row.category = 'sala';
    row.accountCode = 'santander';

    expect(fixture.componentInstance.rowValidation(row).valid).toBe(true);
  });

  it('normalizes a partial category name to the real label on blur', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.type = 'REVENUE';
    row.category = 'sala';

    fixture.componentInstance.finishCategoryEdit(row);

    expect(row.category).toBe('Salário');
  });

  it('expands shorthand day into the previous month when the day is still in the future', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const service = TestBed.inject(TransactionsService);
    const referenceData = TestBed.inject(ReferenceDataService);
    const row = fixture.componentInstance.rows()[0];

    row.description = 'Mercado';
    row.amount = '123,45';
    row.type = 'EXPENSE';
    row.date = '18';
    row.category = 'Cartão';
    row.accountCode = 'santander';

    fixture.componentInstance.submit();

    expect(service.createMany).toHaveBeenCalledWith([
      expect.objectContaining({
        date: '2026-05-18T00:00:00.000Z',
        amount: -12345,
        category_code: 'cartao',
      }),
    ]);
    expect(referenceData.reload).toHaveBeenCalledTimes(1);
  });

  it('sends exclude_from_dashboard for non-transfer rows and clears it for transfers', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const service = TestBed.inject(TransactionsService);
    const firstRow = fixture.componentInstance.rows()[0];
    const secondRow = fixture.componentInstance.rows()[1];

    firstRow.description = 'Mercado';
    firstRow.amount = '123,45';
    firstRow.type = 'EXPENSE';
    firstRow.typeLabel = 'Despesa';
    firstRow.date = '14/06/2026';
    firstRow.category = 'Cartão';
    firstRow.accountCode = 'santander';
    firstRow.excludeFromDashboard = true;

    secondRow.description = 'Pix';
    secondRow.amount = '50,00';
    secondRow.type = 'TRANSFER';
    secondRow.typeLabel = 'Transferência';
    secondRow.date = '14/06/2026';
    secondRow.category = 'Transferencias';
    secondRow.accountCode = 'santander';
    secondRow.transferAccountCode = 'xp';
    secondRow.excludeFromDashboard = true;

    fixture.componentInstance.submit();

    expect(service.createMany).toHaveBeenCalledWith([
      expect.objectContaining({
        exclude_from_dashboard: true,
      }),
      expect.objectContaining({
        is_transfer: true,
        exclude_from_dashboard: false,
      }),
    ]);
  });

  it('shows a compact date label outside edit mode', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.date = '18';

    expect(fixture.componentInstance.compactDateLabel(row)).toBe('18-mai.');
  });

  it('selects the full date when focusing an untouched auto-filled date', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    vi.runAllTimers();
    const component = fixture.componentInstance;
    const row = component.rows()[0];
    const input = fixture.nativeElement.querySelector(
      'input.date-draft-input',
    ) as HTMLInputElement;
    const selectSpy = vi.spyOn(input, 'select');

    row.description = 'Mercado';
    component.onDescriptionChange(row);
    input.value = row.date;
    component.startDateEdit(row, input);
    vi.runAllTimers();

    expect(selectSpy).toHaveBeenCalled();
  });

  it('does not auto-select the date after the user edits it manually', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    vi.runAllTimers();
    const component = fixture.componentInstance;
    const row = component.rows()[0];
    const input = fixture.nativeElement.querySelector(
      'input.date-draft-input',
    ) as HTMLInputElement;
    const selectSpy = vi.spyOn(input, 'select');

    row.description = 'Mercado';
    component.onDescriptionChange(row);
    component.onDateInput(row, '18');
    input.value = row.date;
    component.startDateEdit(row, input);
    vi.runAllTimers();

    expect(selectSpy).not.toHaveBeenCalled();
  });

  it('adds cents when the amount is entered as a whole number', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.amount = '125';
    fixture.componentInstance.finishAmountEdit(row);

    expect(row.amount).toBe('125,00');
  });

  it('shifts cents automatically while typing digits into the amount field', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    fixture.componentInstance.onAmountInput(row, '1');
    expect(row.amount).toBe('0,01');

    fixture.componentInstance.onAmountInput(row, '0,011');
    expect(row.amount).toBe('0,11');

    fixture.componentInstance.onAmountInput(row, '0,111');
    expect(row.amount).toBe('1,11');

    expect(row.amountManualDecimal).toBe(false);
  });

  it('stops auto-shifting after the user explicitly types a comma', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    fixture.componentInstance.handleAmountKeydown(
      new KeyboardEvent('keydown', { key: ',' }),
      row,
    );
    fixture.componentInstance.onAmountInput(row, '123,4');

    expect(row.amount).toBe('123,4');
    expect(row.amountManualDecimal).toBe(true);

    fixture.componentInstance.finishAmountEdit(row);

    expect(row.amount).toBe('123,40');
    expect(row.amountManualDecimal).toBe(true);
  });

  it('spreads pasted tabular data across rows and keeps auto-filled date when the pasted date cell is empty', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();

    const preventDefault = vi.fn();
    fixture.componentInstance.handlePaste(
      {
        clipboardData: {
          getData: () =>
            'Mercado\t125\t\tDespesa\tCartão\tSantander\nPadaria\t18,50\t14\tDespesa\tCartão\tSantander',
        },
        preventDefault,
      } as unknown as ClipboardEvent,
      0,
      0,
    );

    const [firstRow, secondRow] = fixture.componentInstance.rows();

    expect(preventDefault).toHaveBeenCalled();
    expect(firstRow.description).toBe('Mercado');
    expect(firstRow.amount).toBe('125,00');
    expect(firstRow.type).toBe('EXPENSE');
    expect(firstRow.typeLabel).toBe('Despesa');
    expect(firstRow.date).toBe('15/06/2026');
    expect(firstRow.category).toBe('Cartão');
    expect(firstRow.accountCode).toBe('santander');

    expect(secondRow.description).toBe('Padaria');
    expect(secondRow.amount).toBe('18,50');
    expect(secondRow.date).toBe('14/06/2026');
    expect(secondRow.accountCode).toBe('santander');
  });

  it('adds rows when the pasted block is larger than the remaining grid', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();

    fixture.componentInstance.handlePaste(
      {
        clipboardData: {
          getData: () => 'Mercado\t10,00\tDespesa\nPadaria\t12,00\tDespesa',
        },
        preventDefault: vi.fn(),
      } as unknown as ClipboardEvent,
      9,
      0,
    );

    const rows = fixture.componentInstance.rows();
    expect(rows).toHaveLength(11);
    expect(rows[9].description).toBe('Mercado');
    expect(rows[10].description).toBe('Padaria');
  });

  it('builds projected balances from the draft rows', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const [expenseRow, transferRow] = fixture.componentInstance.rows();

    expenseRow.description = 'Mercado';
    expenseRow.amount = '75,00';
    expenseRow.type = 'EXPENSE';
    expenseRow.date = '14';
    expenseRow.category = 'Cartão';
    expenseRow.accountCode = 'santander';

    transferRow.description = 'Aporte';
    transferRow.amount = '100,00';
    transferRow.type = 'TRANSFER';
    transferRow.date = '13';
    transferRow.category = 'Transferencias';
    transferRow.accountCode = 'santander';
    transferRow.transferAccountCode = 'xp';

    expect(fixture.componentInstance.accountPreviewRows()).toEqual([
      {
        code: 'santander',
        name: 'Santander',
        currentBalance: 100000,
        draftImpact: -17500,
        projectedBalance: 82500,
      },
      {
        code: 'xp',
        name: 'XP',
        currentBalance: 50000,
        draftImpact: 10000,
        projectedBalance: 60000,
      },
    ]);
  });

  it('supports arrow-key selection in the category menu', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.type = 'REVENUE';
    fixture.componentInstance.openCategoryMenu(row.id);
    fixture.componentInstance.handleCategoryKeydown(
      new KeyboardEvent('keydown', { key: 'ArrowDown' }),
      row,
    );
    fixture.componentInstance.handleCategoryKeydown(
      new KeyboardEvent('keydown', { key: 'ArrowDown' }),
      row,
    );
    fixture.componentInstance.handleCategoryKeydown(
      new KeyboardEvent('keydown', { key: 'Enter' }),
      row,
    );

    expect(row.category).toBe('Cartão');
  });

  it('supports arrow-key selection in the type menu', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    fixture.componentInstance.openTypeMenu(row.id);
    fixture.componentInstance.handleTypeKeydown(
      new KeyboardEvent('keydown', { key: 'ArrowDown' }),
      row,
    );
    fixture.componentInstance.handleTypeKeydown(
      new KeyboardEvent('keydown', { key: 'Enter' }),
      row,
    );

    expect(row.type).toBe('EXPENSE');
    expect(row.typeLabel).toBe('Despesa');
  });

  it('supports arrow-key selection in the account menu', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    fixture.componentInstance.openAccountMenu(row.id);
    fixture.componentInstance.handleAccountKeydown(
      new KeyboardEvent('keydown', { key: 'ArrowDown' }),
      row,
    );
    fixture.componentInstance.handleAccountKeydown(
      new KeyboardEvent('keydown', { key: 'Enter' }),
      row,
    );

    expect(row.accountCode).toBe('xp');
    expect(row.accountLabel).toBe('XP');
  });

  it('moves to the first cell of the next row on enter, while tab keeps moving across columns', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const host = fixture.nativeElement as HTMLElement;
    const firstRowAmount = host.querySelector(
      'tbody tr:first-child td:nth-child(2) input',
    ) as HTMLInputElement | null;
    const firstRowDate = host.querySelector(
      'tbody tr:first-child td:nth-child(3) input',
    ) as HTMLInputElement | null;
    const secondRowDescription = host.querySelector(
      'tbody tr:nth-child(2) td:first-child input',
    ) as HTMLInputElement | null;

    expect(firstRowAmount).not.toBeNull();
    expect(firstRowDate).not.toBeNull();
    expect(secondRowDescription).not.toBeNull();

    firstRowAmount?.focus();
    firstRowAmount?.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }),
    );

    expect(document.activeElement).toBe(firstRowDate);

    firstRowAmount?.focus();
    firstRowAmount?.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }),
    );

    expect(document.activeElement).toBe(secondRowDescription);
  });

  it('keeps the first visible account match when tabbing out of the field', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const component = fixture.componentInstance;
    const referenceData = TestBed.inject(ReferenceDataService) as unknown as {
      accounts: () => Array<{ Code: string; Name: string; Balance: number }>;
    };
    const row = component.rows()[0];

    vi.spyOn(referenceData, 'accounts').mockReturnValue([
      { Code: 'cartao-santander', Name: 'Cartão Santander', Balance: 100000 },
      { Code: 'cartao-inter', Name: 'Cartão Inter', Balance: 50000 },
      { Code: 'caixa', Name: 'Caixa', Balance: 75000 },
    ]);

    row.accountLabel = 'Ca';
    component.onAccountInputChange(row);

    expect(component.suggestedAccountOptions(row).map((option) => option.label)).toEqual([
      'Cartão Santander',
      'Cartão Inter',
      'Caixa',
    ]);

    component.finishAccountEdit(row);

    expect(row.accountCode).toBe('cartao-santander');
    expect(row.accountLabel).toBe('Cartão Santander');
  });

  it('keeps the first visible category match when tabbing out of the field', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const component = fixture.componentInstance;
    const referenceData = TestBed.inject(ReferenceDataService) as unknown as {
      categories: () => Array<{ Code: string; Name: string; Type: string; SubCategories?: unknown[] }>;
      activeCategories: () => Array<{ Code: string; Name: string; Type: string; SubCategories?: unknown[] }>;
      flatCategories: () => Array<{ Code: string; Name: string; Type: string; ParentID?: number }>;
      activeFlatCategories: () => Array<{ Code: string; Name: string; Type: string; ParentID?: number }>;
    };
    const row = component.rows()[0];

    vi.spyOn(referenceData, 'categories').mockReturnValue([
      { Code: 'cartao-santander', Name: 'Cartão Santander', Type: 'EXPENSE' },
      { Code: 'carro', Name: 'Carro', Type: 'EXPENSE' },
      { Code: 'casa', Name: 'Casa', Type: 'EXPENSE' },
    ]);
    vi.spyOn(referenceData, 'activeCategories').mockReturnValue([
      { Code: 'cartao-santander', Name: 'Cartão Santander', Type: 'EXPENSE' },
      { Code: 'carro', Name: 'Carro', Type: 'EXPENSE' },
      { Code: 'casa', Name: 'Casa', Type: 'EXPENSE' },
    ]);
    vi.spyOn(referenceData, 'flatCategories').mockReturnValue([
      { Code: 'cartao-santander', Name: 'Cartão Santander', Type: 'EXPENSE' },
      { Code: 'carro', Name: 'Carro', Type: 'EXPENSE' },
      { Code: 'casa', Name: 'Casa', Type: 'EXPENSE' },
    ]);
    vi.spyOn(referenceData, 'activeFlatCategories').mockReturnValue([
      { Code: 'cartao-santander', Name: 'Cartão Santander', Type: 'EXPENSE' },
      { Code: 'carro', Name: 'Carro', Type: 'EXPENSE' },
      { Code: 'casa', Name: 'Casa', Type: 'EXPENSE' },
    ]);

    row.type = 'EXPENSE';
    row.category = 'Ca';
    component.onCategoryInputChange(row);

    expect(component.suggestedCategoryOptions(row).map((option) => option.Name)).toEqual([
      'Cartão Santander',
      'Carro',
      'Casa',
    ]);

    component.finishCategoryEdit(row);

    expect(row.category).toBe('Cartão Santander');
  });

  it('evaluates multiple matching suggestions and prefers the longer match on equal priority', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.description = 'Compra no mercado extra do bairro';
    fixture.componentInstance.finishDescriptionEdit(row);

    expect(row.type).toBe('EXPENSE');
    expect(row.category).toBe('Cartão');
    expect(row.accountCode).toBe('santander');
  });

  it('matches suggestions case-insensitively', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const component = fixture.componentInstance;
    const referenceData = TestBed.inject(ReferenceDataService) as unknown as {
      suggestions: () => Array<{
        id: string;
        description_contains: string;
        priority: number;
        entry_type: 'EXPENSE';
        category_code: 'cartao';
        account_code: 'xp';
        transfer_account_code: null;
        created_at: string;
        updated_at: string;
      }>;
    };
    const row = component.rows()[0];

    vi.spyOn(referenceData, 'suggestions').mockReturnValue([
      {
        id: '20',
        description_contains: 'PADARIA',
        priority: 1,
        entry_type: 'EXPENSE',
        category_code: 'cartao',
        account_code: 'xp',
        transfer_account_code: null,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]);

    row.description = 'padaria do centro';
    component.finishDescriptionEdit(row);

    expect(row.accountCode).toBe('xp');
    expect(row.type).toBe('EXPENSE');
  });

  it('matches suggestions accent-insensitively for portuguese text', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const component = fixture.componentInstance;
    const referenceData = TestBed.inject(ReferenceDataService) as unknown as {
      suggestions: () => Array<{
        id: string;
        description_contains: string;
        priority: number;
        entry_type: 'EXPENSE';
        category_code: 'cartao';
        account_code: 'xp';
        transfer_account_code: null;
        created_at: string;
        updated_at: string;
      }>;
    };
    const row = component.rows()[0];

    vi.spyOn(referenceData, 'suggestions').mockReturnValue([
      {
        id: '21',
        description_contains: 'pão de açúcar',
        priority: 1,
        entry_type: 'EXPENSE',
        category_code: 'cartao',
        account_code: 'xp',
        transfer_account_code: null,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]);

    row.description = 'compra no pao de acucar';
    component.finishDescriptionEdit(row);

    expect(row.accountCode).toBe('xp');
    expect(row.type).toBe('EXPENSE');
  });

  it('keeps the suggested category when tabbing through the type field into date editing', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.description = 'mercado da esquina';
    fixture.componentInstance.finishDescriptionEdit(row);

    expect(row.type).toBe('EXPENSE');
    expect(row.typeLabel).toBe('Despesa');
    expect(row.category).toBe('Cartão');

    fixture.componentInstance.finishTypeEdit(row);
    fixture.componentInstance.startDateEdit(row);

    expect(row.type).toBe('EXPENSE');
    expect(row.category).toBe('Cartão');
  });

  it('falls back to fetched array order on an exact tie', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const component = fixture.componentInstance;
    const referenceData = TestBed.inject(ReferenceDataService) as unknown as {
      suggestions: () => Array<{
        id: string;
        description_contains: string;
        priority: number;
        entry_type: 'EXPENSE';
        category_code: 'cartao';
        account_code: string;
        transfer_account_code: null;
        created_at: string;
        updated_at: string;
      }>;
    };
    const row = component.rows()[0];

    vi.spyOn(referenceData, 'suggestions').mockReturnValue([
      {
        id: '10',
        description_contains: 'padaria',
        priority: 1,
        entry_type: 'EXPENSE',
        category_code: 'cartao',
        account_code: 'xp',
        transfer_account_code: null,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
      {
        id: '11',
        description_contains: 'padaria',
        priority: 1,
        entry_type: 'EXPENSE',
        category_code: 'cartao',
        account_code: 'santander',
        transfer_account_code: null,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]);

    row.description = 'Padaria do centro';
    component.finishDescriptionEdit(row);

    expect(row.accountCode).toBe('xp');
  });

  it('fills transfer suggestions and clears transfer fields when a non-transfer suggestion wins later', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.description = 'Pix para reserva';
    fixture.componentInstance.finishDescriptionEdit(row);

    expect(row.type).toBe('TRANSFER');
    expect(row.transferAccountCode).toBe('xp');

    row.description = 'Mercado extra';
    fixture.componentInstance.finishDescriptionEdit(row);

    expect(row.type).toBe('EXPENSE');
    expect(row.transferAccountCode).toBe('');
    expect(row.transferAccountLabel).toBe('');
  });

  it('leaves the row untouched when no suggestion matches', () => {
    const fixture = TestBed.createComponent(InsertTransactionsComponent);
    fixture.detectChanges();
    const row = fixture.componentInstance.rows()[0];

    row.description = 'Assinatura desconhecida';
    row.accountCode = 'xp';
    row.accountLabel = 'XP';
    fixture.componentInstance.finishDescriptionEdit(row);

    expect(row.accountCode).toBe('xp');
    expect(row.type).toBe('');
  });
});
