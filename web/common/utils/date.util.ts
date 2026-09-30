/**
 * Date utility functions for formatting dates in Beijing time (UTC+8)
 */

/**
 * Format date to YYYY-MM-DD HH:mm:ss in Beijing time (UTC+8)
 * @param dateString ISO 8601 date string or Date object
 * @returns Formatted date string in YYYY-MM-DD HH:mm:ss format
 */
export function formatDateToBeijing(dateString: string | Date): string {
  const date = dateString instanceof Date ? dateString : new Date(dateString);
  if (isNaN(date.getTime())) {
    return '';
  }
  // Convert to Beijing time (UTC+8)
  const beijingTime = new Date(date.getTime() + 8 * 60 * 60 * 1000);
  const year = beijingTime.getUTCFullYear();
  const month = String(beijingTime.getUTCMonth() + 1).padStart(2, '0');
  const day = String(beijingTime.getUTCDate()).padStart(2, '0');
  const hours = String(beijingTime.getUTCHours()).padStart(2, '0');
  const minutes = String(beijingTime.getUTCMinutes()).padStart(2, '0');
  const seconds = String(beijingTime.getUTCSeconds()).padStart(2, '0');
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}

/**
 * Format date to YYYY-MM-DDTHH:mm:ss in Beijing time (UTC+8) for datetime-local input
 * @param dateString ISO 8601 date string or Date object
 * @returns Formatted date string in YYYY-MM-DDTHH:mm:ss format
 */
export function formatDateToBeijingForDateTimeLocal(dateString: string | Date): string {
  const formatted = formatDateToBeijing(dateString);
  // Convert YYYY-MM-DD HH:mm:ss to YYYY-MM-DDTHH:mm:ss for datetime-local input
  return formatted.replace(' ', 'T');
}

/**
 * Parse date string in YYYY-MM-DD HH:mm:ss format (treated as Beijing time) to ISO 8601 (UTC)
 * @param dateTimeString Date string in YYYY-MM-DD HH:mm:ss format
 * @returns ISO 8601 date string in UTC
 */
export function parseBeijingDateToISO(dateTimeString: string): string {
  const trimmed = dateTimeString.trim();
  const [datePart, timePart] = trimmed.split(' ');
  if (!datePart || !timePart) {
    throw new Error('Invalid date format. Expected YYYY-MM-DD HH:mm:ss');
  }
  const [year, month, day] = datePart.split('-').map(Number);
  const [hours, minutes, seconds] = timePart.split(':').map(Number);
  // Create date in Beijing time (UTC+8)
  const beijingDate = new Date(Date.UTC(year, month - 1, day, hours, minutes, seconds));
  // Convert to UTC by subtracting 8 hours
  const utcDate = new Date(beijingDate.getTime() - 8 * 60 * 60 * 1000);
  return utcDate.toISOString();
}

/**
 * Get default expired date (current time + 7 days) formatted for input
 * @returns Formatted date string in YYYY-MM-DD HH:mm:ss format
 */
export function getDefaultExpiredDate(): string {
  const defaultExpiredDate = new Date();
  defaultExpiredDate.setDate(defaultExpiredDate.getDate() + 7);
  return formatDateToBeijing(defaultExpiredDate);
}

/**
 * Format date with support for various input types (string, Date, object, null, undefined)
 * This is a unified method that handles all date formatting scenarios
 * @param dateInput Date input in various formats (string, Date, object, null, undefined)
 * @param defaultValue Default value to return if date cannot be formatted (default: '-')
 * @returns Formatted date string in YYYY-MM-DD HH:mm:ss format, or defaultValue if invalid
 */
export function formatDate(dateInput: string | Date | null | undefined | any, defaultValue: string = '-'): string {
  // Handle null/undefined
  if (dateInput == null) {
    return defaultValue;
  }

  // Handle string type
  if (typeof dateInput === 'string') {
    if (dateInput.trim() === '') {
      return defaultValue;
    }
    return formatDateToBeijing(dateInput);
  }

  // If it's already a Date object, convert to ISO string first
  if (dateInput instanceof Date) {
    if (isNaN(dateInput.getTime())) {
      return defaultValue;
    }
    return formatDateToBeijing(dateInput.toISOString());
  }

  // If it's an object, try to extract date string
  if (typeof dateInput === 'object') {
    // Check if it's an empty object
    if (Object.keys(dateInput).length === 0) {
      return defaultValue;
    }

    // Try to find common date string properties
    if (dateInput['$date'] && typeof dateInput['$date'] === 'string') {
      return formatDateToBeijing(dateInput['$date']);
    }

    // Try to find ISO string property
    if (dateInput['toISOString'] && typeof dateInput['toISOString'] === 'function') {
      try {
        return formatDateToBeijing(dateInput.toISOString());
      } catch (e) {
        return defaultValue;
      }
    }

    // Try to find time property (for Go time.Time serialization)
    if (dateInput['Time'] && typeof dateInput['Time'] === 'string') {
      return formatDateToBeijing(dateInput['Time']);
    }

    // Try to find any string property that looks like a date
    for (const key in dateInput) {
      if (typeof dateInput[key] === 'string' && dateInput[key].match(/^\d{4}-\d{2}-\d{2}/)) {
        return formatDateToBeijing(dateInput[key]);
      }
    }

    // Last resort: try to stringify and parse
    try {
      const jsonStr = JSON.stringify(dateInput);
      const parsed = JSON.parse(jsonStr);
      if (typeof parsed === 'string' && parsed.match(/^\d{4}-\d{2}-\d{2}/)) {
        return formatDateToBeijing(parsed);
      }
    } catch (e) {
      // Ignore JSON errors
    }
  }

  return defaultValue;
}

