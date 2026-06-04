export interface MessageApi {
  success(text: string): void
  error(text: string): void
  warning(text: string): void
  info(text: string): void
}
