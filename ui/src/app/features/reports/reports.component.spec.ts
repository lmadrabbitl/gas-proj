import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { Router } from '@angular/router';
import { of, Subject } from 'rxjs';
import { vi } from 'vitest';

import { ReferenceDataService } from '../../data/reference-data.service';
import { ReportsService } from '../../data/reports.service';
import { UserConfigService } from '../../data/user-config.service';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { ReportsComponent } from './reports.component';

describe('ReportsComponent', () => {
  const yearlyData = [
    { code: 'salario', monthly_data: [300000, 320000, 310000], subcategories: [] },
    {
      code: 'alimentacao',
      monthly_data: [-10000, -12000, -9000],
      subcategories: [
        {
          code: 'mercado',
          monthly_data: [-10000, -12000, -9000],
          top_items_by_month: [
            [
              { description: 'Compra do mes', amount: -7000 },
              { description: 'Reposicao rapida', amount: -3000 },
            ],
            [
              { description: 'Feira grande', amount: -12000 },
            ],
            null,
          ],
          subcategories: [],
        },
        { code: 'lanche', monthly_data: [0, 0, 0], top_items_by_month: [null, null, null], subcategories: [] },
      ],
    },
    {
      code: 'lazer',
      monthly_data: [0, 0, 0],
      subcategories: [
        { code: 'cinema', monthly_data: [0, 0, 0], top_items_by_month: [null, null, null], subcategories: [] },
      ],
    },
  ];
  const yearly = vi.fn();
  const navigate = vi.fn();
  const reportsConfig = signal({ show_empty_categories: true });
  const updateReportsConfig = vi.fn();
  const config = signal({
    language: 'pt-BR',
    settings: {
      transactions: {
        list: {
          page_size: 50,
          show_total: false,
        },
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

  beforeEach(async () => {
    yearly.mockReset();
    yearly.mockReturnValue(of(yearlyData));
    navigate.mockReset();
    updateReportsConfig.mockReset();
    updateReportsConfig.mockImplementation((config: { show_empty_categories: boolean }) => {
      reportsConfig.set(config);
      return of(undefined);
    });
    reportsConfig.set({ show_empty_categories: true });
    config.set({
      language: 'pt-BR',
      settings: {
        transactions: {
          list: {
            page_size: 50,
            show_total: false,
          },
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

    await TestBed.configureTestingModule({
      imports: [ReportsComponent],
      providers: [
        {
          provide: ReportsService,
          useValue: {
            yearly,
          },
        },
        {
          provide: Router,
          useValue: {
            navigate,
          },
        },
        {
          provide: ReferenceDataService,
          useValue: {
            load: () => of(void 0),
            categoryName: (code: string) => ({
              salario: 'Salário',
              alimentacao: 'Alimentação',
              mercado: 'Mercado',
              lanche: 'Lanche',
              lazer: 'Lazer',
              cinema: 'Cinema',
            })[code] ?? code,
            flatCategories: () => [
              { Code: 'salario', Type: 'INCOME' },
              { Code: 'alimentacao', Type: 'EXPENSE' },
              { Code: 'mercado', Type: 'EXPENSE' },
              { Code: 'lanche', Type: 'EXPENSE' },
              { Code: 'lazer', Type: 'EXPENSE' },
              { Code: 'cinema', Type: 'EXPENSE' },
            ],
          },
        },
        {
          provide: UserConfigService,
          useValue: {
            config,
            reportsConfig,
            updateReportsConfig,
            syncReportsConfig: vi.fn().mockImplementation((config: { show_empty_categories: boolean }) => {
              reportsConfig.set(config);
            }),
          },
        },
        {
          provide: MoneyVisibilityService,
          useValue: {
            formatCompactCurrencyAbsolute: (value: number) => `R$ ${Math.abs(value)}`,
          },
        },
      ],
    }).compileComponents();
  });

  it('renders the report tables after loading', async () => {
    const fixture = TestBed.createComponent(ReportsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    expect(host.textContent).toContain('Mensal por categoria');
    expect(host.textContent).toContain('Salário');
    expect(host.textContent).toContain('Alimentação');
    expect(host.textContent).toContain('Lazer');
    expect(host.querySelector('.income-report-table')).not.toBeNull();
    expect(host.querySelector('.expense-report-table')).not.toBeNull();
    expect(host.querySelector('.summary-report-table')).not.toBeNull();
  });

  it('shows hover details only on child category cells with culprit items', async () => {
    const fixture = TestBed.createComponent(ReportsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const detailCells = host.querySelectorAll('.report-detail-cell');

    expect(detailCells.length).toBe(2);
    expect(host.textContent).toContain('Compra do mes');
    expect(host.textContent).toContain('Reposicao rapida');
    expect(host.textContent).toContain('Feira grande');
    expect(host.textContent).toContain('Top 5 Mercado · Janeiro/26');
    expect(host.textContent).toContain('Top 5 Mercado · Fevereiro/26');
    expect(host.querySelector('.summary-report-table .report-detail-cell')).toBeNull();
    expect([...host.querySelectorAll('.parent-report-row .report-detail-cell')]).toHaveLength(0);
  });

  it('navigates to the transactions page with category and month filters when a detail cell is clicked', async () => {
    const fixture = TestBed.createComponent(ReportsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const firstDetailCell = host.querySelector('.report-detail-cell') as HTMLElement;

    firstDetailCell.click();

    expect(navigate).toHaveBeenCalledWith(['/transactions'], {
      queryParams: {
        category_code: 'mercado',
        from_date: '2026-01-01',
        to_date: '2026-01-31',
      },
    });
  });

  it('hides empty child and parent expense categories when configured', async () => {
    reportsConfig.set({ show_empty_categories: false });

    const fixture = TestBed.createComponent(ReportsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    expect(host.textContent).toContain('Mercado');
    expect(host.textContent).not.toContain('Lanche');
    expect(host.textContent).not.toContain('Cinema');
    expect(host.textContent).not.toContain('Lazer');
  });

  it('shows report skeletons while loading and disables year navigation', async () => {
    const response$ = new Subject<typeof yearlyData>();
    yearly.mockReturnValueOnce(response$.asObservable());

    const fixture = TestBed.createComponent(ReportsComponent);
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const buttons = host.querySelectorAll('.report-year-switcher > button');

    expect(host.querySelector('[data-testid="reports-skeleton"]')).not.toBeNull();
    expect(host.textContent).toContain('Receitas e investimentos');
    expect(host.textContent).toContain('Despesas');
    expect(host.textContent).toContain('Resumo');
    expect((buttons[0] as HTMLButtonElement).disabled).toBe(true);
    expect((buttons[1] as HTMLButtonElement).disabled).toBe(true);

    response$.next(yearlyData);
    response$.complete();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(host.querySelector('[data-testid="reports-skeleton"]')).toBeNull();
    expect(host.textContent).toContain('Resumo');
  });

  it('lets the user jump directly to a selected year from the middle picker', async () => {
    const fixture = TestBed.createComponent(ReportsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    yearly.mockClear();

    const host = fixture.nativeElement as HTMLElement;
    const middleButton = host.querySelector('.report-year-pill') as HTMLButtonElement;

    middleButton.click();
    fixture.detectChanges();

    const yearButton = [...host.querySelectorAll('.report-year-option')]
      .find((node) => node.textContent?.trim() === '2020') as HTMLButtonElement | undefined;

    expect(yearButton).toBeDefined();

    yearButton?.click();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(yearly).toHaveBeenCalledTimes(1);
    expect(yearly).toHaveBeenCalledWith(2020);
  });
});
