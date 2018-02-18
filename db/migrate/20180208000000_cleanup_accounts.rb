require Rails.root.join('lib', 'mastodon', 'migration_helpers')

class CleanupAccounts < ActiveRecord::Migration[5.1]
  include Mastodon::MigrationHelpers

  def change
    safety_assured do
      trigger_name = rename_trigger_name(:accounts, :suspension, :halted)
      check_trigger_permissions!(:accounts)
      remove_rename_triggers_for_postgresql(:accounts, trigger_name)
    end
  end
end
