import { SignIn, SignUp } from '@clerk/clerk-react';

export function SignInPage() {
  return (
    <div className="auth-wrapper">
      <SignIn routing="path" path="/sign-in" signUpUrl="/sign-up" />
    </div>
  );
}

export function SignUpPage() {
  return (
    <div className="auth-wrapper">
      <SignUp routing="path" path="/sign-up" signInUrl="/sign-in" />
    </div>
  );
}

