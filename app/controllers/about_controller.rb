# frozen_string_literal: true

class AboutController < ApplicationController
  before_action :set_body_classes
  before_action :set_csp
  before_action :set_instance_presenter, only: [:show, :more]

  def show; end

  def more; end

  def terms; end

  private

  def new_user
    User.new.tap(&:build_account)
  end
  helper_method :new_user

  def set_instance_presenter
    @instance_presenter = InstancePresenter.new
  end

  def set_body_classes
    @body_classes = 'about-body'
  end

  def set_csp
    response.headers['Content-Security-Policy'] = "default-src 'none'; font-src #{ContentSecurityPolicy::ASSET}; img-src #{ContentSecurityPolicy::ASSET}; script-src #{ContentSecurityPolicy::ASSET}; style-src #{ContentSecurityPolicy::ASSET}"
  end
end
