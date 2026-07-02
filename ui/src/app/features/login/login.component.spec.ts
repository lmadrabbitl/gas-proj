import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { LoginComponent } from './login.component';

describe('LoginComponent', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    localStorage.clear();
    await TestBed.configureTestingModule({
      imports: [LoginComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()],
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    http.verify();
    localStorage.clear();
  });

  it('renders login fields and account creation toggle', () => {
    const fixture = TestBed.createComponent(LoginComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.querySelector('.login-brand lucide-icon')).toBeTruthy();
    expect(compiled.querySelector('input[type="email"]')).toBeTruthy();
    expect(compiled.querySelector('input[type="password"]')).toBeTruthy();
    expect(compiled.textContent).toContain('Criar conta');
    expect(compiled.textContent).not.toContain('Criar usuario teste');
  });

  it('shows the name field in register mode', () => {
    const fixture = TestBed.createComponent(LoginComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;

    const buttons = Array.from(compiled.querySelectorAll('button'));
    const registerButton = buttons.find((button) => button.textContent?.includes('Criar conta'));
    registerButton?.click();
    fixture.detectChanges();

    const nameInput = fixture.nativeElement.querySelector('input[formControlName="name"]') as HTMLInputElement | null;
    expect(nameInput).toBeTruthy();
  });

  it('returns to login mode with a success message after registration', () => {
    const fixture = TestBed.createComponent(LoginComponent);
    fixture.detectChanges();

    const component = fixture.componentInstance;
    component.setMode('register');
    component.form.setValue({
      name: 'Test User',
      email: 'test@user.com',
      password: 'password123',
    });
    fixture.detectChanges();

    component.submit();

    const req = http.expectOne('/api/auth/register');
    expect(req.request.method).toBe('POST');
    req.flush({
      user: { id: '123', name: 'Test User', email: 'test@user.com' },
    });
    fixture.detectChanges();

    expect(component.mode()).toBe('login');
    expect(component.info()).toContain('Conta criada com sucesso');
    expect(component.form.controls.email.value).toBe('test@user.com');
  });
});
