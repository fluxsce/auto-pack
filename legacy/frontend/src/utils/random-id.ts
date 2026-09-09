/** UUID v4–style id without `crypto.randomUUID` (works on HTTP / older runtimes). */
export function randomId(): string {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (ch) => {
    const n = Math.floor(Math.random() * 16)
    const v = ch === 'x' ? n : (n & 0x3) | 0x8
    return v.toString(16)
  })
}
