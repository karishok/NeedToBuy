import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// @testing-library/react's built-in auto-cleanup only registers itself when
// `afterEach` is a true global, which this project doesn't enable (no
// `test.globals: true`). Register it explicitly so each test starts with a
// clean DOM instead of accumulating markup from previous tests in the file.
afterEach(() => {
  cleanup()
})
