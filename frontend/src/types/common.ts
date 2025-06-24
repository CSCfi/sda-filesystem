export type ValidationIconType = "success" | "error" | "info";

export type ValidationResult = {
  lowerCaseOrNum: boolean;
  inputLength: boolean;
  alphaNumDash: boolean;
  unique: boolean;
};
