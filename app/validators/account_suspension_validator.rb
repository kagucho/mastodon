# frozen_string_literal: true

class AccountSuspensionValidator < ActiveModel::Validator
  def validate(account)
    user.errors.add(:base, 'Account must be suspended before halted') if account.halted && !account.suspended
  end
end
