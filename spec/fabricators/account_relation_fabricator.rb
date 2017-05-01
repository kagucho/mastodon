Fabricator(:account_relation) do
  account
  target_account { Fabricate(:account) }
end
