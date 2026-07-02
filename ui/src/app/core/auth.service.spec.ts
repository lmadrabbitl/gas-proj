import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { AuthService } from './auth.service';

describe('AuthService', () => {
  let service: AuthService;
  let http: HttpTestingController;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(AuthService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    http.verify();
    localStorage.clear();
  });

  it('stores the token after login', () => {
    service.login('test@user.com', 'password123').subscribe();

    const req = http.expectOne('/api/auth/login');
    expect(req.request.method).toBe('POST');
    req.flush({ token: 'abc123' });

    expect(service.token()).toBe('abc123');
    expect(service.isLoggedIn()).toBe(true);
  });

  it('removes the token on logout', () => {
    service.login('test@user.com', 'password123').subscribe();
    http.expectOne('/api/auth/login').flush({ token: 'abc123' });

    service.logout();

    expect(service.token()).toBeNull();
    expect(service.isLoggedIn()).toBe(false);
  });

  it('registers without storing a token', () => {
    service.register({ name: 'Test User', email: 'test@user.com', password: 'password123' }).subscribe();

    const req = http.expectOne('/api/auth/register');
    expect(req.request.method).toBe('POST');
    req.flush({
      user: { id: '123', name: 'Test User', email: 'test@user.com' },
    });

    expect(service.token()).toBeNull();
    expect(service.isLoggedIn()).toBe(false);
  });
});
