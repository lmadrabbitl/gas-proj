import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { vi } from 'vitest';

import { AccountsService } from '../../data/accounts.service';
import { ReferenceDataService } from '../../data/reference-data.service';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { AccountsComponent } from './accounts.component';

describe('AccountsComponent', () => {
  const accountsSignal = signal([
    {
      ID: '1',
      UserID: 'user-1',
      Code: 'btg',
      Name: 'BTG',
      Type: 'ASSET' as const,
      Balance: 100000,
      Currency: 'BRL',
      hide_from_dashboard: true,
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
      DeactivatedAt: null,
    },
    {
      ID: '2',
      UserID: 'user-1',
      Code: 'nubank',
      Name: 'Nubank',
      Type: 'LIABILITY' as const,
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
      Code: 'conta-teste',
      Name: 'Conta Teste',
      Type: 'ASSET' as const,
      Balance: 0,
      Currency: 'BRL',
      hide_from_dashboard: false,
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
      DeactivatedAt: '2026-06-01T12:00:00Z',
    },
    {
      ID: '4',
      UserID: 'user-1',
      Code: 'conta-teste-2',
      Name: 'conta teste',
      Type: 'ASSET' as const,
      Balance: 0,
      Currency: 'BRL',
      hide_from_dashboard: false,
      CreatedAt: '2026-06-02T00:00:00Z',
      UpdatedAt: '2026-06-02T00:00:00Z',
      DeactivatedAt: null,
    },
  ]);

  const accountsService = {
    list: vi.fn().mockReturnValue(of(accountsSignal())),
    create: vi.fn().mockReturnValue(of(accountsSignal()[0])),
    update: vi.fn().mockReturnValue(of(accountsSignal()[0])),
    reorder: vi.fn().mockReturnValue(of(void 0)),
    deactivate: vi.fn().mockReturnValue(of(void 0)),
    deletePermanent: vi.fn().mockReturnValue(of(void 0)),
  };

  const referenceData = {
    accounts: accountsSignal,
    accountName: (code: string) => {
      const account = accountsSignal().find((candidate) => candidate.Code === code);
      if (!account) {
        return code;
      }
      if (!account.DeactivatedAt) {
        return account.Name;
      }

      const normalized = account.Name.trim().toLocaleLowerCase('pt-BR');
      const hasCollision = accountsSignal().some(
        (candidate) =>
          candidate.Code !== account.Code &&
          candidate.Name.trim().toLocaleLowerCase('pt-BR') === normalized,
      );
      if (!hasCollision) {
        return account.Name;
      }

      return `${account.Name} (desativada em 01/06/2026)`;
    },
    reload: vi.fn().mockReturnValue(of(void 0)),
  };

  const moneyVisibility = {
    formatCurrency: (value: number) => `R$ ${value / 100}`,
  };

  beforeEach(async () => {
    accountsService.list.mockClear();
    accountsService.create.mockClear();
    accountsService.update.mockClear();
    accountsService.reorder.mockClear();
    accountsService.deactivate.mockClear();
    accountsService.deletePermanent.mockClear();
    referenceData.reload.mockClear();

    await TestBed.configureTestingModule({
      imports: [AccountsComponent],
      providers: [
        { provide: AccountsService, useValue: accountsService },
        { provide: ReferenceDataService, useValue: referenceData },
        { provide: MoneyVisibilityService, useValue: moneyVisibility },
      ],
    }).compileComponents();
  });

  it('shows dashboard visibility in the table', async () => {
    const fixture = TestBed.createComponent(AccountsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Oculta');
  });

  it('sends hide_from_dashboard when updating an account', () => {
    const fixture = TestBed.createComponent(AccountsComponent);
    const component = fixture.componentInstance;
    const account = accountsSignal()[0];

    component.openEdit(account);
    component.form.patchValue({ hide_from_dashboard: false });
    component.save();

    expect(accountsService.update).toHaveBeenCalledWith('btg', expect.objectContaining({
      hide_from_dashboard: false,
    }));
  });

  it('creates an account without sending a code', () => {
    const fixture = TestBed.createComponent(AccountsComponent);
    const component = fixture.componentInstance;

    component.openCreate();
    component.form.patchValue({
      name: 'Bradesco',
      type: 'ASSET',
      currency: 'BRL',
      hide_from_dashboard: true,
    });
    component.save();

    expect(accountsService.create).toHaveBeenCalledWith({
      name: 'Bradesco',
      type: 'ASSET',
      currency: 'BRL',
      hide_from_dashboard: true,
    });
  });

  it('adds a liability class to liability rows', async () => {
    const fixture = TestBed.createComponent(AccountsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const rows = [...(fixture.nativeElement as HTMLElement).querySelectorAll('tbody tr')];
    const liabilityRow = rows.find((row) => row.textContent?.includes('Nubank'));

    expect(liabilityRow?.classList.contains('liability-row')).toBe(true);
  });

  it('shows the disambiguated name for deactivated reused accounts', async () => {
    const fixture = TestBed.createComponent(AccountsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect((fixture.nativeElement as HTMLElement).textContent).toContain(
      'Conta Teste (desativada em 01/06/2026)',
    );
  });

  it('removes the order and dashboard columns from the inactive accounts table and shows a delete action', async () => {
    const fixture = TestBed.createComponent(AccountsComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const headings = [...host.querySelectorAll('.table-section:last-of-type thead th')].map((node) =>
      node.textContent?.trim(),
    );

    expect(headings).not.toContain('Ordem');
    expect(headings).not.toContain('Visível no dashboard');
    expect(
      host.querySelector(
        '.table-section:last-of-type button[aria-label="Excluir conta permanentemente"]',
      ),
    ).not.toBeNull();
  });

  it('permanently deletes an inactive account after confirmation', () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const fixture = TestBed.createComponent(AccountsComponent);
    const component = fixture.componentInstance;

    component.deletePermanently(accountsSignal()[2]);

    expect(accountsService.deletePermanent).toHaveBeenCalledWith('conta-teste');
    confirmSpy.mockRestore();
  });
});
