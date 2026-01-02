import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useForm } from '@tanstack/react-form'
import { z } from 'zod'
import { toast } from 'sonner'
import { authClient } from '@/lib/rpc'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { useState } from 'react'

export const Route = createFileRoute('/register')({
  component: RegisterPage,
})

const registerSchema = z.object({
  email: z.string().email('Invalid email address'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  fullName: z.string().min(1, 'Full name is required'),
  countryCode: z
    .string()
    .length(2, 'Country code must be 2 characters')
    .regex(/^[A-Z]{2}$/, 'Must be uppercase ISO 3166-1 alpha-2'),
  phoneNumber: z.string().min(1, 'Phone number is required'),
})

function RegisterPage() {
  const navigate = useNavigate()
  const [serverError, setServerError] = useState<string | null>(null)

  const form = useForm({
    defaultValues: {
      email: '',
      password: '',
      fullName: '',
      countryCode: '',
      phoneNumber: '',
    },
    validators: {
      onChange: registerSchema,
    },
    onSubmit: async ({ value }) => {
      setServerError(null)
      try {
        await authClient.register({
          email: value.email,
          password: value.password,
          fullName: value.fullName,
          countryCode: value.countryCode,
          phoneNumber: value.phoneNumber,
        })

        toast.success('Registration successful! You are now logged in.')
        navigate({ to: '/' })
      } catch (error) {
        console.error('Registration failed', error)
        const errorMessage =
          error instanceof Error ? error.message : 'Registration failed'
        toast.error(errorMessage)
        setServerError(errorMessage)
      }
    },
  })

  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900 p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>Create an Account</CardTitle>
          <CardDescription>
            Enter your details to register for the auction system.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              e.stopPropagation()
              form.handleSubmit()
            }}
            className="space-y-4"
          >
            <form.Field
              name="email"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    placeholder="john@example.com"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors ? (
                    <p className="text-sm text-red-500">
                      {field.state.meta.errors.join(', ')}
                    </p>
                  ) : null}
                </div>
              )}
            />

            <form.Field
              name="password"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="password">Password</Label>
                  <Input
                    id="password"
                    type="password"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors ? (
                    <p className="text-sm text-red-500">
                      {field.state.meta.errors.join(', ')}
                    </p>
                  ) : null}
                </div>
              )}
            />

            <form.Field
              name="fullName"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="fullName">Full Name</Label>
                  <Input
                    id="fullName"
                    placeholder="John Doe"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors ? (
                    <p className="text-sm text-red-500">
                      {field.state.meta.errors.join(', ')}
                    </p>
                  ) : null}
                </div>
              )}
            />

            <form.Field
              name="countryCode"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="countryCode">
                    Country Code (2-letter ISO)
                  </Label>
                  <Input
                    id="countryCode"
                    placeholder="US"
                    maxLength={2}
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) =>
                      field.handleChange(e.target.value.toUpperCase())
                    }
                  />
                  {field.state.meta.errors ? (
                    <p className="text-sm text-red-500">
                      {field.state.meta.errors.join(', ')}
                    </p>
                  ) : null}
                </div>
              )}
            />

            <form.Field
              name="phoneNumber"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="phoneNumber">Phone Number</Label>
                  <Input
                    id="phoneNumber"
                    placeholder="+1234567890"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors ? (
                    <p className="text-sm text-red-500">
                      {field.state.meta.errors.join(', ')}
                    </p>
                  ) : null}
                </div>
              )}
            />

            {serverError && (
              <div className="text-sm text-red-500 font-medium">
                {serverError}
              </div>
            )}

            <form.Subscribe
              selector={(state) => [state.canSubmit, state.isSubmitting]}
              children={([canSubmit, isSubmitting]) => (
                <Button
                  type="submit"
                  className="w-full"
                  disabled={!canSubmit || isSubmitting}
                >
                  {isSubmitting ? 'Registering...' : 'Register'}
                </Button>
              )}
            />
          </form>
        </CardContent>
        <CardFooter className="justify-center">
          <p className="text-sm text-gray-500">
            Already have an account?{' '}
            <a href="/auth/login" className="text-blue-500 hover:underline">
              Log in
            </a>
          </p>
        </CardFooter>
      </Card>
    </div>
  )
}
