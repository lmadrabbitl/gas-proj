import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';

import { AccountsService } from './accounts.service';
import { CategoriesService } from './categories.service';
import { ReferenceDataService } from './reference-data.service';
import { SuggestionsService } from './suggestions.service';

describe('ReferenceDataService', () => {
  let service: ReferenceDataService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        ReferenceDataService,
        { provide: AccountsService, useValue: { list: () => of([]) } },
        { provide: CategoriesService, useValue: { list: () => of([]) } },
        { provide: SuggestionsService, useValue: { list: () => of([]) } },
      ],
    });

    service = TestBed.inject(ReferenceDataService);
  });

  it('disambiguates deactivated accounts when the name was reused', () => {
    service.accounts.set([
      {
        ID: '1',
        UserID: 'user-1',
        Code: 'bradesco',
        Name: 'Bradesco',
        Type: 'ASSET',
        Balance: 0,
        Currency: 'BRL',
        hide_from_dashboard: false,
        CreatedAt: '2026-01-01T00:00:00Z',
        UpdatedAt: '2026-01-01T00:00:00Z',
        DeactivatedAt: '2026-06-01T12:00:00Z',
      },
      {
        ID: '2',
        UserID: 'user-1',
        Code: 'bradesco-2',
        Name: 'Bradesco',
        Type: 'ASSET',
        Balance: 0,
        Currency: 'BRL',
        hide_from_dashboard: false,
        CreatedAt: '2026-06-02T00:00:00Z',
        UpdatedAt: '2026-06-02T00:00:00Z',
        DeactivatedAt: null,
      },
    ]);

    expect(service.accountName('bradesco')).toBe('Bradesco (desativada em 01/06/2026)');
    expect(service.accountName('bradesco-2')).toBe('Bradesco');
  });

  it('falls back to the code when the account is unknown', () => {
    expect(service.accountName('conta-antiga')).toBe('conta-antiga');
  });

  it('disambiguates deactivated accounts when the reused name differs only by case', () => {
    service.accounts.set([
      {
        ID: '1',
        UserID: 'user-1',
        Code: 'conta-teste',
        Name: 'Conta Teste',
        Type: 'ASSET',
        Balance: 0,
        Currency: 'BRL',
        hide_from_dashboard: false,
        CreatedAt: '2026-01-01T00:00:00Z',
        UpdatedAt: '2026-01-01T00:00:00Z',
        DeactivatedAt: '2026-06-01T12:00:00Z',
      },
      {
        ID: '2',
        UserID: 'user-1',
        Code: 'conta-teste-2',
        Name: 'conta teste',
        Type: 'ASSET',
        Balance: 0,
        Currency: 'BRL',
        hide_from_dashboard: false,
        CreatedAt: '2026-06-02T00:00:00Z',
        UpdatedAt: '2026-06-02T00:00:00Z',
        DeactivatedAt: null,
      },
    ]);

    expect(service.accountName('conta-teste')).toBe('Conta Teste (desativada em 01/06/2026)');
    expect(service.accountName('conta-teste-2')).toBe('conta teste');
  });
});
