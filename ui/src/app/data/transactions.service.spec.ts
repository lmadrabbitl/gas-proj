import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { TransactionPayload } from '../shared/models';
import { TransactionsService } from './transactions.service';

describe('TransactionsService', () => {
  let service: TransactionsService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(TransactionsService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('sends many transactions using the backend batch shape', () => {
    const transactions: TransactionPayload[] = [
      {
        date: '2026-01-02T00:00:00.000Z',
        description: 'Mercado',
        amount: -12345,
        account_code: 'santander',
        category_code: 'supermercado',
        is_transfer: false,
        account_transfer: null,
        exclude_from_dashboard: false,
      },
    ];

    service.createMany(transactions).subscribe();

    const req = http.expectOne('/api/transactions');
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ transactions });
    req.flush({ transactions: [] });
  });

  it('serializes filters and drops one-character description terms', () => {
    service.list({
      account_code: ['santander', 'xp'],
      category_code: ['supermercado', 'lazer'],
      operation: ['credit', 'transfer'],
      description: 'de -juros dividendo i -a',
    }).subscribe();

    const req = http.expectOne((request) => request.url === '/api/transactions');
    expect(req.request.method).toBe('GET');
    expect(req.request.params.get('account_code')).toBe('santander,xp');
    expect(req.request.params.get('category_code')).toBe('supermercado,lazer');
    expect(req.request.params.get('operation')).toBe('credit,transfer');
    expect(req.request.params.get('description')).toBe('de -juros dividendo');
    req.flush({
      transactions: [],
      pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 },
      config: { page_size: 50, show_total: false },
    });
  });

  it('omits the description param when every term is only one character', () => {
    service.list({
      description: 'i -a',
    }).subscribe();

    const req = http.expectOne((request) => request.url === '/api/transactions');
    expect(req.request.method).toBe('GET');
    expect(req.request.params.has('description')).toBe(false);
    req.flush({
      transactions: [],
      pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 },
      config: { page_size: 50, show_total: false },
    });
  });

  it('sends bulk transaction updates to the dedicated endpoint', () => {
    service.updateMany({
      ids: ['tx-1', 'tx-2'],
      category_code: 'mercado',
      is_transfer: false,
    }).subscribe();

    const req = http.expectOne('/api/transactions/bulk');
    expect(req.request.method).toBe('PATCH');
    expect(req.request.body).toEqual({
      ids: ['tx-1', 'tx-2'],
      category_code: 'mercado',
      is_transfer: false,
    });
    req.flush({ updated_count: 2 });
  });

  it('sends bulk transaction deletes to the dedicated endpoint', () => {
    service.deleteMany(['tx-1', 'tx-2']).subscribe();

    const req = http.expectOne('/api/transactions/bulk-delete');
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ ids: ['tx-1', 'tx-2'] });
    req.flush({});
  });
});
