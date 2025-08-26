export type ValidationIconType = "success" | "error" | "info";

export type ValidationHelperType = {
  check: string;
  message: string;
  type: ValidationIconType;
};

type ValidationType = boolean | undefined;

export type ValidationResult = {
  lowerCaseOrNum: ValidationType;
  inputLength: ValidationType;
  alphaNumHyphen: ValidationType;
  ownable: ValidationType;
};

export type TabType = "Log in" | "Access" | "Export" | "Logs";
