# frozen_string_literal: true

class HomeController < ApplicationController
  include JavascriptEntry

  def index
    @body_classes = 'app-body'
  end
end
