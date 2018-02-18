require Rails.root.join('lib', 'mastodon', 'migration_helpers')

class AddHaltedToAccounts < ActiveRecord::Migration[5.1]
  include Mastodon::MigrationHelpers

  disable_ddl_transaction!

  def change
    safety_assured { rename_column_concurrently :accounts, :suspended, :halted }
  end
end
